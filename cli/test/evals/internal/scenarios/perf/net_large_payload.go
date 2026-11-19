package perf

import (
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/net-large-payload", pncSeedLargePayload) }

// pncMobileUA reports whether a request came from a phone (the iOS web view
// or the Android app's okhttp).
func pncMobileUA(r *seed.Req) bool {
	ua, _ := r.Request().Root.Attrs["user_agent.original"].(string)
	return strings.Contains(ua, "iPhone") || strings.HasPrefix(ua, "okhttp")
}

// pncSeedLargePayload: the feed endpoint returns 50 cards with their images
// inlined as data URIs, ~2.3MB per response. The query and the handler's
// own work take ~30ms; the rest of the request is the body going out, so
// latency follows http.response.body.size and the client's bandwidth (phones
// take seconds, desktops a few hundred ms). Distractor: the feed query runs
// on every request with no cache in front of it, but it takes ~15ms.
func pncSeedLargePayload(s *seed.Scenario) error {
	window := 2 * 24 * time.Hour
	p := s.Project("social", seed.OTel)
	api := p.Service("feed-api", seed.Go("nethttp", "sql")).Versions("8.4.2")

	feed := api.Endpoint("GET", "/api/feed")
	feed.Traffic(seed.Daily(2400), seed.Diurnal(), seed.Over(window))
	feed.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 10})
	feed.Handler(func(r *seed.Req) {
		withImages := 40 + r.IntN(11)
		size := 58000 + r.IntN(9000) + withImages*(44000+r.IntN(4000))
		r.RootAttr("http.response.body.size", size)
		r.Gap(r.Ms(0.3, 0.9))
		r.DB("SELECT c.id, c.kind, c.title, c.body, c.image_data, c.published_at, a.id, a.display_name, a.avatar_data FROM feed_cards c JOIN authors a ON a.id = c.author_id WHERE c.audience = ANY($1) ORDER BY c.published_at DESC LIMIT $2",
			r.LogNormal(14*time.Millisecond, 32*time.Millisecond), seed.Rows(50))
		r.Compute("rank cards", r.Ms(2, 6)).Scope = "feed-api"
		// json.NewEncoder(w).Encode writes the body as it encodes; on a
		// slow link the writes block until the client has read it.
		mbps := 40 + 110*r.Float()
		if pncMobileUA(r) {
			mbps = 5 + 15*r.Float()
		}
		r.Gap(r.Ms(14, 30) + time.Duration(float64(size)*8/(mbps*1e6)*float64(time.Second)))
	})

	card := api.Endpoint("GET", "/api/feed/cards/{id}")
	card.Traffic(seed.Daily(900), seed.Diurnal(), seed.Over(window))
	card.Handler(func(r *seed.Req) {
		size := 2200 + r.IntN(1800)
		if r.Chance(0.8) {
			size += 44000 + r.IntN(4000)
		}
		r.RootAttr("http.response.body.size", size)
		r.Gap(r.Ms(0.3, 0.9))
		r.DB("SELECT c.id, c.kind, c.title, c.body, c.image_data, c.published_at, a.id, a.display_name FROM feed_cards c JOIN authors a ON a.id = c.author_id WHERE c.id = $1", r.LogNormal(2*time.Millisecond, 5*time.Millisecond))
		mbps := 40 + 110*r.Float()
		if pncMobileUA(r) {
			mbps = 5 + 15*r.Float()
		}
		r.Gap(r.Ms(0.5, 1.5) + time.Duration(float64(size)*8/(mbps*1e6)*float64(time.Second)))
	})

	authors := api.Endpoint("GET", "/api/authors/{id}")
	authors.Traffic(seed.Daily(600), seed.Diurnal(), seed.Over(window))
	authors.Handler(func(r *seed.Req) {
		r.RootAttr("http.response.body.size", 900+r.IntN(700))
		r.Gap(r.Ms(0.3, 0.9))
		r.DB("SELECT id, display_name, bio, followers FROM authors WHERE id = $1", r.LogNormal(2*time.Millisecond, 5*time.Millisecond))
		r.Gap(r.Ms(0.3, 1))
	})

	s.Background(api, seed.TrafficOpts{Endpoints: 3, PerDay: 150})
	api.Host().CPU(seed.Flat(0.17, 0.26))
	api.Host().Memory(seed.Flat(0.44, 0.52))

	isMobile := func(r *seed.Request) bool {
		ua, _ := r.Root.Attrs["user_agent.original"].(string)
		return strings.Contains(ua, "iPhone") || strings.HasPrefix(ua, "okhttp")
	}
	s.Expose("mobileRequest", feed.Pick(func(r *seed.Request) bool {
		return isMobile(r) && r.Start.After(s.Ago(3*time.Hour)) && r.Duration() > 1500*time.Millisecond
	}))
	s.Expose("desktopRequest", feed.Pick(func(r *seed.Request) bool {
		return !isMobile(r) && r.Start.After(s.Ago(3*time.Hour)) && r.Duration() < 500*time.Millisecond
	}))
	s.Expose("p50Ms24h", func() any { return feed.Percentile(0.5, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p95Ms24h", func() any { return feed.Percentile(0.95, 24*time.Hour).Seconds() * 1000 })
	return nil
}
