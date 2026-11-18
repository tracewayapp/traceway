package perf

import (
	"math"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/app-quadratic", papSeedQuadratic) }

// papSeedQuadratic: POST /api/quotes prices a cart; the apply_promotions span
// costs ~0.011ms x items^2, so a 40-item cart takes ~20ms and a 350-item
// wholesale cart well over a second. The two queries are fast and constant.
// Distractor: a large in-memory allocation; but process RSS and host memory
// stay flat, and the time is pure compute that grows with the square of
// cart.items, not with any data volume the service holds.
func papSeedQuadratic(s *seed.Scenario) error {
	p := s.Project("shop", seed.OTel)
	api := p.Service("pricing-service", seed.Python("fastapi", "psycopg")).Versions("3.8.1")
	window := 3 * 24 * time.Hour

	quotes := api.Endpoint("POST", "/api/quotes")
	quotes.Traffic(seed.Daily(1700), seed.Diurnal(), seed.Over(window))
	quotes.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.04, Max: 50})
	quotes.Handler(func(r *seed.Req) {
		items := int(math.Min(450, math.Max(1, float64(r.LogNormal(12*time.Millisecond, 330*time.Millisecond).Milliseconds()))))
		n := float64(items)
		r.RootAttr("cart.items", items)
		r.Gap(r.Ms(0.6, 1.4) + time.Duration(n*0.012*float64(time.Millisecond))) // request validation
		r.DB("SELECT sku, family, unit_price_cents, tax_class FROM prices WHERE sku = ANY(%(skus)s)",
			r.Ms(2.5, 6)+time.Duration(n*0.01*float64(time.Millisecond)), seed.Rows(items))
		r.DB("SELECT id, kind, family, sku, partner_sku, min_quantity, percent, amount_cents FROM promotions WHERE active AND starts_at <= now() AND ends_at > now()",
			r.Ms(1.5, 3.5), seed.Rows(18))
		promo := time.Duration((0.011*n*n + 0.04*n) * (0.92 + 0.16*r.Float()) * float64(time.Millisecond))
		r.Compute("apply_promotions", promo, seed.A("promotions.active", 18)).Scope = "app.pricing"
		r.Compute("calculate_tax", r.Ms(0.2, 0.6)+time.Duration(n*0.004*float64(time.Millisecond))).Scope = "app.pricing"
		r.Gap(r.Ms(0.4, 1.2) + time.Duration(n*0.006*float64(time.Millisecond))) // response model
	})

	carts := api.Endpoint("GET", "/api/price-lists/{price_list_id}")
	carts.Traffic(seed.Daily(700), seed.Diurnal(), seed.Over(window))
	carts.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.2))
		r.DB("SELECT sku, family, unit_price_cents, tax_class FROM prices WHERE price_list_id = %(price_list_id)s ORDER BY sku LIMIT 200", r.Ms(4, 11), seed.Rows(200))
		r.Gap(r.Ms(1.5, 3.5))
	})

	s.Background(api, seed.TypicalTraffic(8))
	api.Host().CPU(seed.Flat(0.21, 0.31))
	api.Host().Memory(seed.Flat(0.44, 0.49))
	api.Gauge("process.runtime.cpython.memory", "By", map[string]any{"type": "rss"}, seed.Flat(212e6, 228e6))
	api.Gauge("process.runtime.cpython.memory", "By", map[string]any{"type": "vms"}, seed.Flat(612e6, 640e6))

	pick := func(lo, hi int) func() any {
		return quotes.Pick(func(rq *seed.Request) bool {
			v, _ := rq.Root.Attrs["cart.items"].(int)
			return rq.Start.After(s.Ago(12*time.Hour)) && v >= lo && v <= hi
		})
	}
	s.Expose("bigQuote", pick(330, 450))
	s.Expose("midQuote", pick(150, 190))
	s.Expose("smallQuote", pick(35, 50))
	s.Expose("p50Ms24h", func() any { return quotes.Percentile(0.5, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p95Ms24h", func() any { return quotes.Percentile(0.95, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p99Ms24h", func() any { return quotes.Percentile(0.99, 24*time.Hour).Seconds() * 1000 })
	return nil
}
