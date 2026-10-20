package perf

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/cache-no-cache", pncSeedNoCache) }

const pncWebOrigin = "https://shop.example.com"

var pncWebPages = []string{"/c/sneakers", "/c/boots", "/c/sandals", "/c/running", "/c/outdoor", "/c/kids", "/c/sale", "/c/new-in"}

// pncSeedNoCache: the shop's web app fetches the whole catalog (~410KB,
// 1200 products) again on every route change, with a cache-busting `_=`
// timestamp, and catalog-api keys its Redis cache by the full URL. Every
// request misses (cache.hit_ratio for the catalog cache ~3%), runs the
// catalog query and serializes 1200 products, so GET /api/catalog is by far
// the busiest endpoint for a catalog that changes twice a day. The browser
// project's occurrences carry the captured navigations and fetches that
// show the repeat. Distractor: the catalog query is the long pole of each
// request (~45ms), which reads like a slow query, but it is steady and the
// cost is volume times repetition.
func pncSeedNoCache(s *seed.Scenario) error {
	window := 24 * time.Hour
	api := s.Project("shop", seed.OTel)
	web := s.Project("shop-web", seed.Browser, "react")
	s.Expose("primaryProject", "shop")
	svc := api.Service("catalog-api", seed.Node("express", "pg")).Versions("6.2.1")

	catalog := svc.Endpoint("GET", "/api/catalog")
	catalog.Traffic(seed.Daily(9000), seed.Diurnal(), seed.Over(window))
	catalog.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.002, Max: 8})
	catalog.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		url := "/api/catalog?locale=en-US"
		hit := r.Chance(0.03) // the prerender job calls without the buster
		if !hit {
			url += fmt.Sprintf("&_=%d", r.Start().UnixMilli()-int64(r.IntN(400)))
		}
		r.RootAttr("url.query", url[len("/api/catalog?"):])
		if hit {
			r.Cache("get", "catalog:"+url, r.Ms(0.6, 1.6))
			r.RootAttr("http.response.body.size", 412366)
			r.Gap(r.Ms(1.5, 3))
			return
		}
		r.Cache("get", "catalog:"+url, r.Ms(0.4, 1.2))
		r.DB("SELECT p.id, p.sku, p.title, p.brand, p.image_url, c.slug AS category, pr.amount_minor, pr.currency FROM products p JOIN categories c ON c.id = p.category_id JOIN prices pr ON pr.product_id = p.id AND pr.price_list = $1 WHERE p.active ORDER BY c.position, p.rank",
			r.LogNormal(44*time.Millisecond, 95*time.Millisecond), seed.Rows(1180+r.IntN(40)))
		ser := r.Compute("serialize catalog", r.Ms(6, 12))
		ser.Scope = "catalog-api"
		r.Cache("set", "catalog:"+url+" [3 other arguments]", r.Ms(0.4, 1.1))
		r.RootAttr("http.response.body.size", 412366)
		r.Gap(r.Ms(0.3, 0.8))
	})

	reviews := svc.Endpoint("GET", "/api/products/:sku/reviews")
	reviews.Traffic(seed.Daily(1300), seed.Diurnal(), seed.Over(window))
	reviews.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		r.Cache("get", "sess:"+fmt.Sprintf("%x", r.IntN(1<<30)), r.Ms(0.3, 0.9))
		r.DB("SELECT id, rating, title, body, created_at FROM reviews WHERE product_sku = $1 ORDER BY created_at DESC LIMIT 10 OFFSET $2", r.LogNormal(4*time.Millisecond, 10*time.Millisecond), seed.Rows(10))
		r.Gap(r.Ms(0.3, 1)) // res.json
	})

	s.Background(svc, seed.TrafficOpts{Endpoints: 3, PerDay: 150})
	svc.Host().CPU(seed.Flat(0.34, 0.46))
	svc.Host().Memory(seed.Flat(0.52, 0.6))
	svc.Gauge("cache.hit_ratio", "1", map[string]any{"cache": "catalog"}, seed.Flat(0.022, 0.045))
	svc.Gauge("cache.hit_ratio", "1", map[string]any{"cache": "session"}, seed.Flat(0.955, 0.985))

	// The web app: sessions, and the AbortError a quick route change raises
	// when it aborts the in-flight catalog fetch. Each occurrence carries the
	// navigations and fetches the SDK captured before it.
	const bundle = pncWebOrigin + "/assets/index-DqK3x8aP.js"
	abortStack := "AbortError: signal is aborted without reason\n    at " + bundle + ":48:20113\n    at " + bundle + ":48:19871"
	uas := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1",
	}
	rng := s.RNG
	catalogURL := func(at time.Time) string {
		return fmt.Sprintf("%s/api/catalog?locale=en-US&_=%d", pncWebOrigin, at.UnixMilli())
	}
	sessionAttrs := func(ua, path string) map[string]string {
		platform, viewport, screen := "Win32", "1440x812", "1920x1080"
		switch {
		case strings.Contains(ua, "iPhone"):
			platform, viewport, screen = "iPhone", "390x664", "390x844"
		case strings.Contains(ua, "Macintosh"):
			platform, viewport, screen = "MacIntel", "1512x823", "1512x982"
		}
		return map[string]string{"url": pncWebOrigin + path, "path": path, "referrer": "", "userAgent": ua, "language": "en-US",
			"platform": platform, "viewport": viewport, "screen": screen, "timezone": "Europe/Berlin"}
	}
	for i := 0; i < 380; i++ {
		start := s.Ago(time.Duration(30+rng.IntN(23*60)) * time.Minute).Add(time.Duration(rng.IntN(60)) * time.Second)
		ua := uas[rng.IntN(len(uas))]
		web.Session(uuid.NewString(), start, start.Add(time.Duration(3+rng.IntN(25))*time.Minute), sessionAttrs(ua, "/"), "2026.10.3")
	}
	var firstAt time.Time
	for i := 0; i < 9; i++ {
		at := s.Ago(time.Duration(25+i*61+rng.IntN(20)) * time.Minute).Add(time.Duration(rng.IntN(60)) * time.Second)
		if i == 0 {
			firstAt = at
		}
		sid := uuid.NewString()
		ua := uas[rng.IntN(len(uas))]
		started := at.Add(-time.Duration(4+rng.IntN(10)) * time.Minute)
		web.Session(sid, started, at.Add(time.Duration(2+rng.IntN(6))*time.Minute), sessionAttrs(ua, "/"), "2026.10.3")

		var acts []seed.Action
		from := "/"
		t := at.Add(-time.Duration(40+rng.IntN(15)) * time.Second)
		for step := 0; step < 4; step++ {
			to := pncWebPages[rng.IntN(len(pncWebPages))]
			if step == 1 {
				to = fmt.Sprintf("/p/%s", fmt.Sprintf("SH-%05d", 1000+rng.IntN(80000)))
			}
			acts = append(acts, seed.Action{Type: "navigation", At: t, From: from, To: to})
			acts = append(acts, seed.Action{Type: "network", At: t.Add(30 * time.Millisecond), Method: "GET", URL: catalogURL(t.Add(30 * time.Millisecond)),
				DurationMs: float64(62 + rng.IntN(60)), Status: 200, RespBytes: 412366})
			if step == 1 {
				acts = append(acts, seed.Action{Type: "network", At: t.Add(45 * time.Millisecond), Method: "GET",
					URL: pncWebOrigin + "/api/products" + to[2:] + "/reviews?page=1", DurationMs: float64(25 + rng.IntN(25)), Status: 200, RespBytes: 9000 + rng.IntN(3000)})
			}
			from = to
			t = t.Add(time.Duration(7+rng.IntN(9)) * time.Second)
		}
		// A quick second click aborts the fetch the previous route started.
		last := pncWebPages[rng.IntN(len(pncWebPages))]
		acts = append(acts, seed.Action{Type: "navigation", At: at.Add(-900 * time.Millisecond), From: from, To: last})
		acts = append(acts, seed.Action{Type: "network", At: at.Add(-870 * time.Millisecond), Method: "GET", URL: catalogURL(at.Add(-870 * time.Millisecond)),
			DurationMs: 860, Error: "AbortError: signal is aborted without reason"})
		next := fmt.Sprintf("/p/BT-%05d", 1000+rng.IntN(80000))
		acts = append(acts, seed.Action{Type: "navigation", At: at.Add(-10 * time.Millisecond), From: last, To: next})
		acts = append(acts, seed.Action{Type: "network", At: at, Method: "GET", URL: catalogURL(at), DurationMs: float64(60 + rng.IntN(50)), Status: 200, RespBytes: 412366})
		web.Crash(seed.Crash{At: at, Stack: abortStack, AppVersion: "2026.10.3", SessionID: sid,
			Attributes: map[string]string{"url": pncWebOrigin + next, "userAgent": ua}, Actions: acts})
	}

	s.Expose("slowRequest", catalog.Pick(func(r *seed.Request) bool {
		q, _ := r.Root.Attrs["url.query"].(string)
		return r.Start.After(s.Ago(3*time.Hour)) && len(q) > len("locale=en-US")
	}))
	s.Expose("crashAt", firstAt.UTC().Format(time.RFC3339Nano))
	s.Expose("catalogCount24h", func() any { return catalog.Count(24 * time.Hour) })
	s.AfterFlush(func() error {
		hash, err := web.GroupMatching(24*time.Hour, "signal is aborted")
		if err != nil {
			return err
		}
		s.Expose("webHash", hash)
		return nil
	})
	return nil
}
