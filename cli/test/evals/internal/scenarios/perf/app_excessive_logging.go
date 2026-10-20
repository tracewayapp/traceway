package perf

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/app-excessive-logging", papSeedExcessiveLogging) }

// papSeedExcessiveLogging: release 5.3.1 of listings-api, deployed 26h ago,
// only switches LOG_LEVEL to debug. From then on every request writes ~15-20x
// as many log lines (one per decoded row and resolved image), each costing
// ~0.5ms on the request path, so every endpoint gets slower at the deploy and
// host CPU steps from ~35% to ~78% five minutes later, once the rollout has
// finished. Traffic is flat. Distractor: CPU saturation; but the CPU rise
// follows the deploy, request counts did not change, and the extra time per
// request is spent writing logs.
func papSeedExcessiveLogging(s *seed.Scenario) error {
	p := s.Project("marketplace", seed.OTel)
	deployAt := s.Ago(26*time.Hour - 9*time.Minute)
	api := p.Service("listings-api", seed.Go("nethttp", "pgx")).
		Versions("5.3.0", seed.Deploy{At: deployAt, Version: "5.3.1"}).
		Instances("listings-api-6c9f7d8b5-k2x4q", "listings-api-6c9f7d8b5-p8n2m")
	window := 48 * time.Hour
	debug := func(r *seed.Req) bool { return !r.Start().Before(deployAt) }
	// logLine writes a debug line when the release logs at debug level, and
	// pays its cost on the request path.
	logLine := func(r *seed.Req, body string, attrs ...seed.KV) {
		if debug(r) {
			r.Log(seed.SevDebug, body, attrs...)
			r.Gap(r.Ms(0.35, 0.75))
		}
	}

	search := api.Endpoint("GET", "/api/listings")
	search.Traffic(papJitter("search", 0.07), seed.Daily(650), seed.Diurnal(), seed.Over(window))
	search.Handler(func(r *seed.Req) {
		r.KeepDetail() // every request has its access log line
		r.Gap(r.Ms(0.4, 1.0))
		category := 100 + r.IntN(400)
		key := fmt.Sprintf("listings:search:%d:%x", category, r.IntN(1<<20))
		logLine(r, "cache lookup", seed.A("cache.key", key))
		r.Cache("GET", key, r.Ms(0.3, 0.9))
		rows := 36 + r.IntN(9)
		r.DB("SELECT id, seller_id, title, price_cents, currency, thumbnail_url, ranked_at FROM listings WHERE category_id = $1 AND status = 'active' ORDER BY ranked_at DESC LIMIT 44",
			r.Ms(6, 15), seed.Rows(rows))
		for i := 0; i < rows; i++ {
			logLine(r, "decoded listing", seed.A("listing.id", 300000+r.IntN(600000)), seed.A("seller.id", 1000+r.IntN(90000)))
		}
		logLine(r, "query executed", seed.A("db.table", "listings"), seed.A("rows", rows))
		r.Gap(r.Ms(0.3, 0.9)) // marshal the page for the cache
		r.Cache("SET", key, r.Ms(0.3, 0.9))
		r.Log(seed.SevInfo, "search served", seed.A("category.id", category), seed.A("results", rows))
		r.Gap(r.Ms(0.5, 1.5)) // JSON encode
	})

	show := api.Endpoint("GET", "/api/listings/{id}")
	show.Traffic(papJitter("show", 0.07), seed.Daily(600), seed.Diurnal(), seed.Over(window))
	show.Handler(func(r *seed.Req) {
		r.KeepDetail()
		r.Gap(r.Ms(0.4, 1.0))
		var id int
		path, _ := r.Request().Root.Attrs["url.path"].(string)
		fmt.Sscanf(path, "/api/listings/%d", &id)
		logLine(r, "cache lookup", seed.A("cache.key", fmt.Sprintf("listing:%d", id)))
		r.Cache("GET", fmt.Sprintf("listing:%d", id), r.Ms(0.3, 0.9))
		r.DB("SELECT id, seller_id, title, description, price_cents, currency, status, ranked_at FROM listings WHERE id = $1", r.Ms(1.5, 4), seed.Rows(1))
		logLine(r, "query executed", seed.A("db.table", "listings"), seed.A("rows", 1))
		images := 12 + r.IntN(10)
		r.DB("SELECT url, width, height, position FROM listing_images WHERE listing_id = $1 ORDER BY position", r.Ms(1.5, 4), seed.Rows(images))
		logLine(r, "query executed", seed.A("db.table", "listing_images"), seed.A("rows", images))
		logLine(r, "decoded listing", seed.A("listing.id", id))
		for i := 0; i < images; i++ {
			logLine(r, "image url resolved", seed.A("listing.id", id), seed.A("position", i))
		}
		r.Gap(r.Ms(0.5, 1.5))
	})

	views := api.Endpoint("POST", "/api/listings/{id}/views")
	views.Traffic(papJitter("views", 0.07), seed.Daily(350), seed.Diurnal(), seed.Over(window))
	views.Handler(func(r *seed.Req) {
		r.KeepDetail()
		r.Gap(r.Ms(0.3, 0.8))
		logLine(r, "recording view")
		r.DB("INSERT INTO listing_views (listing_id, viewer_id, viewed_at) VALUES ($1, $2, now())", r.Ms(1.5, 4))
		logLine(r, "query executed", seed.A("db.table", "listing_views"), seed.A("rows", 1))
		r.Status(204)
	})

	cats := api.Endpoint("GET", "/api/categories")
	cats.Traffic(papJitter("cats", 0.07), seed.Daily(200), seed.Diurnal(), seed.Over(window))
	cats.Handler(func(r *seed.Req) {
		r.KeepDetail()
		r.Gap(r.Ms(0.3, 0.8))
		logLine(r, "cache lookup", seed.A("cache.key", "categories:v3"))
		r.Cache("GET", "categories:v3", r.Ms(0.3, 0.8))
		logLine(r, "cache hit", seed.A("cache.key", "categories:v3"))
		r.Gap(r.Ms(0.6, 1.6))
	})

	rolled := deployAt.Add(5 * time.Minute)
	for i, h := range api.Hosts {
		api.HostNamed(h).CPU(seed.Step(rolled, seed.Flat(0.3, 0.38), seed.Flat(0.74, 0.82)))
		api.HostNamed(h).Memory(seed.Flat(0.38, 0.43))
		api.LogOn(h, deployAt.Add(time.Duration(i)*2*time.Minute+40*time.Second), seed.SevInfo, "starting listings-api",
			seed.A("version", "5.3.1"), seed.A("log.level", "debug"), seed.A("addr", ":8080"))
	}

	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	// Windows at the same time of day before and after the release, so the
	// diurnal traffic curve does not confound the comparison.
	rfc := func(t time.Time) string { return t.Format(time.RFC3339) }
	s.Expose("beforeFrom", rfc(deployAt.Add(-6*time.Hour)))
	s.Expose("afterFrom", rfc(deployAt.Add(18*time.Hour)))
	s.Expose("afterTo", rfc(deployAt.Add(24*time.Hour)))
	s.Expose("beforeHourFrom", rfc(deployAt.Add(-3*time.Hour)))
	s.Expose("beforeHourTo", rfc(deployAt.Add(-2*time.Hour)))
	s.Expose("afterHourFrom", rfc(deployAt.Add(21*time.Hour)))
	s.Expose("afterHourTo", rfc(deployAt.Add(22*time.Hour)))
	s.Expose("beforeRequest", search.Pick(func(rq *seed.Request) bool {
		return rq.Start.Before(deployAt.Add(-2*time.Hour)) && rq.Start.After(deployAt.Add(-4*time.Hour))
	}))
	s.Expose("afterRequest", search.Pick(func(rq *seed.Request) bool { return rq.Start.After(s.Ago(6 * time.Hour)) }))
	s.Expose("searchP95Before", func() any {
		return search.PercentileBetween(0.95, deployAt.Add(-6*time.Hour), deployAt).Seconds() * 1000
	})
	s.Expose("searchP95After", func() any {
		return search.PercentileBetween(0.95, deployAt.Add(18*time.Hour), deployAt.Add(24*time.Hour)).Seconds() * 1000
	})
	return nil
}
