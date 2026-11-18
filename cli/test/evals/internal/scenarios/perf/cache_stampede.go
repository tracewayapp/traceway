package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/cache-stampede", pncSeedStampede) }

// pncHomeSections are the queries that rebuild the home feed, with their
// share of a rebuild's DB time.
var pncHomeSections = []struct {
	q string
	w float64
}{
	{"SELECT id, image_url, target_url, starts_at, ends_at FROM banners WHERE placement = 'home' AND now() BETWEEN starts_at AND ends_at ORDER BY priority", 0.03},
	{"SELECT c.id, c.title, c.slug, c.hero_image FROM collections c WHERE c.featured ORDER BY c.position LIMIT 6", 0.04},
	{"SELECT ol.sku, SUM(ol.qty) AS sold FROM order_lines ol JOIN orders o ON o.id = ol.order_id WHERE o.placed_at > now() - interval '7 days' GROUP BY ol.sku ORDER BY sold DESC LIMIT 24", 0.31},
	{"SELECT id, sku, title, price_minor, image_url FROM products WHERE published_at > now() - interval '14 days' ORDER BY published_at DESC LIMIT 24", 0.07},
	{"SELECT p.id, p.sku, p.title, p.price_minor, p.image_url FROM editorial_picks e JOIN products p ON p.id = e.product_id WHERE e.week = date_trunc('week', now()) ORDER BY e.position", 0.04},
	{"SELECT term, count(*) AS n FROM search_events WHERE created_at > now() - interval '1 day' GROUP BY term ORDER BY n DESC LIMIT 10", 0.18},
	{"SELECT id, slug, title, tile_image FROM categories WHERE parent_id IS NULL ORDER BY position", 0.02},
	{"SELECT d.id, d.product_id, d.discount_pct, d.ends_at FROM deals d WHERE d.active AND d.ends_at > now() ORDER BY d.discount_pct DESC LIMIT 12", 0.05},
	{"SELECT b.id, b.name, b.logo_url FROM brands b WHERE b.spotlight_until > now() LIMIT 4", 0.02},
	{"SELECT r.product_id, avg(r.rating) AS rating, count(*) AS n FROM reviews r WHERE r.created_at > now() - interval '3 days' GROUP BY r.product_id ORDER BY n DESC LIMIT 12", 0.14},
	{"SELECT id, title, image_url FROM seasonal_stories WHERE season = $1 ORDER BY position LIMIT 3", 0.03},
	{"SELECT sku FROM product_scores WHERE segment = 'default' ORDER BY score DESC LIMIT 24", 0.07},
}

// pncSeedStampede: GET /api/home serves the home feed from Redis
// (home:feed:v3, 10-minute TTL). When the key expires, every request that
// arrives before the first rebuild has written it back misses and rebuilds
// the feed itself: ~2.4 req/s times a ~7s rebuild under that contention is
// 15-30 identical rebuild traces per expiry, every ~10 minutes. Between
// expiries the hit ratio is ~98%. Distractor: the misses run 12 queries
// against the DB on the request path, which reads like a missing cache, but
// the cache is there and hits ~98% of the time.
func pncSeedStampede(s *seed.Scenario) error {
	window := 2 * time.Hour
	p := s.Project("storefront-home", seed.OTel)
	api := p.Service("home-api", seed.Go("nethttp", "sql")).Versions("3.4.1")

	// Expiry schedule: each window [at, at+fill) is when the key is gone
	// and requests miss; the first rebuild's SET closes it and the next
	// expiry is ttl later. Built over 24h so the hit-ratio gauge shows it
	// all day.
	type expiry struct {
		at   time.Time
		fill time.Duration
	}
	var expiries []expiry
	for t := s.Ago(24*time.Hour + 7*time.Minute + 23*time.Second); t.Before(s.Now); {
		fill := time.Duration(7000+s.RNG.IntN(3000)) * time.Millisecond
		expiries = append(expiries, expiry{t, fill})
		t = t.Add(fill + 10*time.Minute)
	}
	missWindow := func(t time.Time) (expiry, bool) {
		for _, e := range expiries {
			if !t.Before(e.at) && t.Before(e.at.Add(e.fill)) {
				return e, true
			}
		}
		return expiry{}, false
	}
	var recent []time.Time // the last two expiries inside the traffic window
	for i := len(expiries) - 1; i >= 0 && len(recent) < 2; i-- {
		if expiries[i].at.Add(expiries[i].fill).Before(s.Now.Add(-time.Minute)) {
			recent = append(recent, expiries[i].at)
		}
	}

	misses := map[*seed.Request]bool{}
	home := api.Endpoint("GET", "/api/home")
	home.Traffic(seed.Daily(207360), seed.Over(window)) // 2.4 req/s
	home.Detail(seed.DetailPolicy{Recent: window, Rate: 0.0015, Max: 12})
	home.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.2, 0.6))
		e, miss := missWindow(r.Start())
		if !miss {
			r.Cache("get", "home:feed:v3", r.LogNormal(700*time.Microsecond, 2*time.Millisecond))
			r.Compute("decode feed", r.Ms(0.8, 2.2)).Scope = "home-api"
			r.Gap(r.Ms(0.3, 1.2))
			return
		}
		misses[r.Request()] = true
		for _, at := range recent {
			if at.Equal(e.at) {
				r.KeepDetail()
			}
		}
		r.Cache("get", "home:feed:v3", r.LogNormal(700*time.Microsecond, 2*time.Millisecond))
		total := float64(e.fill) * (0.88 + 0.24*r.Float())
		r.Span("buildHomeFeed", func(r *seed.Req) {
			for _, sec := range pncHomeSections {
				r.DB(sec.q, time.Duration(total*sec.w*(0.9+0.2*r.Float())))
			}
			r.Compute("encode feed", r.Ms(6, 14)).Scope = "home-api"
		}).Scope = "home-api"
		r.Cache("set", "home:feed:v3", r.Ms(0.6, 1.8), seed.A("db.query.text", "set home:feed:v3 ? ex 600"))
		r.Gap(r.Ms(0.3, 1.2))
	})

	product := api.Endpoint("GET", "/api/home/stories/{id}")
	product.Traffic(seed.Daily(6000), seed.Over(window))
	product.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.2, 0.6))
		r.DB("SELECT id, title, body, image_url, published_at FROM seasonal_stories WHERE id = $1", r.LogNormal(2500*time.Microsecond, 7*time.Millisecond))
		r.Compute("encode response", r.Ms(0.3, 1)).Scope = "home-api"
	})

	s.Background(api, seed.TrafficOpts{Endpoints: 3, PerDay: 900, Window: window})
	api.Host().CPU(seed.Flat(0.21, 0.3))
	api.Host().Memory(seed.Flat(0.4, 0.47))
	api.Gauge("cache.hit_ratio", "1", map[string]any{"cache": "home-feed"}, func(t time.Time) float64 {
		for _, e := range expiries {
			if !t.Before(e.at) && t.Before(e.at.Add(time.Minute)) {
				return 0.89 - 0.03*float64(e.fill)/float64(10*time.Second)
			}
		}
		return 0.984
	})

	s.Expose("missRequest", home.Pick(func(r *seed.Request) bool { return misses[r] && len(recent) > 0 && r.Start.After(recent[0]) }))
	if len(recent) > 0 {
		s.Expose("lastExpiry", recent[0].UTC().Format(time.RFC3339))
		s.Expose("spikeFrom", recent[0].Truncate(5*time.Minute).UTC().Format(time.RFC3339))
		s.Expose("spikeTo", recent[0].Add(time.Minute).Truncate(5*time.Minute).Add(5*time.Minute).UTC().Format(time.RFC3339))
	}
	s.Expose("missesPerExpiry", func() any {
		counts := map[time.Time]int{}
		for r := range misses {
			if e, ok := missWindow(r.Start); ok {
				counts[e.at]++
			}
		}
		out := []int{}
		for _, e := range expiries {
			if n, ok := counts[e.at]; ok && e.at.After(s.Ago(window)) {
				out = append(out, n)
			}
		}
		return out
	})
	s.Expose("p50Ms2h", func() any { return home.Percentile(0.5, window).Seconds() * 1000 })
	s.Expose("p99Ms2h", func() any { return home.Percentile(0.99, window).Seconds() * 1000 })
	return nil
}
