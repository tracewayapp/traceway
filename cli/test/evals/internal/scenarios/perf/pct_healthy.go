package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/ctl-healthy", pctSeedHealthy) }

// pctSeedHealthy: GET /api/search is healthy and has been for eight days
// (p50 ~23ms, p95 ~50ms, no errors, no growth, flat CPU). Expected:
// none_found. Distractors: the search query uses ILIKE on name/brand, which
// looks like it wants a trigram index but takes ~15ms; checkout is the
// slowest endpoint (a payment call), unrelated to search.
func pctSeedHealthy(s *seed.Scenario) error {
	p := s.Project("shop", seed.OTel)
	window := 8 * 24 * time.Hour
	api := p.Service("storefront-api", seed.Node("express", "pg")).Versions("1.42.0", seed.Deploy{At: s.Ago(5*24*time.Hour + 5*time.Hour), Version: "1.43.0"})

	search := api.Endpoint("GET", "/api/search")
	search.Traffic(seed.Daily(600), seed.Diurnal(), seed.Over(window))
	search.Detail(seed.DetailPolicy{Recent: 24 * time.Hour, Rate: 0.05, Max: 20})
	search.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.6, 1.6))
		r.DB("SELECT id, name, slug, brand, price_cents, thumbnail_url FROM products WHERE active AND (name ILIKE $1 OR brand ILIKE $1) ORDER BY popularity DESC LIMIT 24",
			r.LogNormal(14*time.Millisecond, 55*time.Millisecond), seed.Rows(1+r.IntN(24)))
		r.DB("SELECT facet, value, count(*) AS n FROM product_facets WHERE product_id = ANY($1) GROUP BY facet, value",
			r.LogNormal(4*time.Millisecond, 15*time.Millisecond), seed.Rows(6+r.IntN(30)))
		r.Gap(r.Ms(1, 4))
		r.Gap(r.Ms(0.3, 0.9))
	})

	product := api.Endpoint("GET", "/api/products/:id")
	product.Traffic(seed.Daily(420), seed.Diurnal(), seed.Over(window))
	product.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.6, 1.6))
		r.DB("SELECT id, name, slug, brand, description, price_cents, images FROM products WHERE id = $1 AND active", r.LogNormal(3*time.Millisecond, 10*time.Millisecond), seed.Rows(1))
		r.DB("SELECT sku, size, color, stock FROM product_variants WHERE product_id = $1", r.LogNormal(2*time.Millisecond, 8*time.Millisecond), seed.Rows(4+r.IntN(12)))
		r.Gap(r.Ms(0.5, 2))
	})
	checkout := api.Endpoint("POST", "/api/checkout")
	checkout.Traffic(seed.Daily(80), seed.Diurnal(), seed.Over(window))
	checkout.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.8, 2))
		r.DB("BEGIN", r.Ms(0.2, 0.6))
		r.DB("SELECT id, items, total_cents, currency FROM carts WHERE id = $1 FOR UPDATE", r.LogNormal(3*time.Millisecond, 10*time.Millisecond))
		r.HTTP("POST", "https://api.payments.example/v1/payment_intents", r.LogNormal(380*time.Millisecond, 900*time.Millisecond), 200)
		r.DB("INSERT INTO orders (cart_id, total_cents, currency, payment_ref, created_at) VALUES ($1, $2, $3, $4, now()) RETURNING id", r.LogNormal(4*time.Millisecond, 12*time.Millisecond))
		r.DB("COMMIT", r.Ms(0.8, 2.5))
		r.Status(201)
	})
	s.Background(api, seed.TypicalTraffic(6))
	api.Host().CPU(seed.Flat(0.17, 0.27))
	api.Host().Memory(seed.Flat(0.46, 0.54))

	s.Expose("slowRequest", search.Slowest(24*time.Hour))
	s.Expose("typicalRequest", pctRef(func() *seed.Request {
		return pctNearestBetween(search, s.Ago(24*time.Hour), s.Now, search.Percentile(0.5, 24*time.Hour))
	}))
	s.Expose("p50Ms24h", pctLazyMs(search, 0.5, s.Ago(24*time.Hour), s.Now))
	s.Expose("p95Ms24h", pctLazyMs(search, 0.95, s.Ago(24*time.Hour), s.Now))
	s.Expose("p99Ms24h", pctLazyMs(search, 0.99, s.Ago(24*time.Hour), s.Now))
	s.Expose("p95MsFirstDays", pctLazyMs(search, 0.95, s.Ago(window), s.Ago(window-3*24*time.Hour)))
	return nil
}
