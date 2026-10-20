// Package perf holds the seed functions of the perf suite: one scenario per
// bottleneck checklist row, plus controls and cross-surface cases.
package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/db-n-plus-1", seedNPlusOne) }

// seedNPlusOne: the orders endpoint loads a customer's orders, then one
// order_items query per order. Customers accumulate orders over the 29 days,
// so latency climbs with no deploy and flat CPU. Distractor: the orders query
// selects every column (over-fetching), but it is fast.
func seedNPlusOne(s *seed.Scenario) error {
	p := s.Project("storefront", seed.OTel)
	api := p.Service("orders-api", seed.Go("nethttp", "sql")).Versions("2.14.3")
	window := 29 * 24 * time.Hour

	orders := api.Endpoint("GET", "/api/orders")
	orders.Traffic(seed.Daily(160), seed.Diurnal(), seed.Over(window))
	orders.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.25, Max: 12})
	orders.Handler(func(r *seed.Req) {
		// Orders per customer grow over the window; LIMIT 100 caps them.
		n := int(r.Lerp(6, 72) * (0.35 + 0.95*r.Float()))
		if n > 100 {
			n = 100
		}
		if n < 1 {
			n = 1
		}
		r.Gap(r.Ms(0.4, 1.1)) // auth middleware
		r.DB("SELECT * FROM orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT 100", r.Ms(3, 6), seed.Rows(n))
		for range n {
			r.DB("SELECT id, sku, qty, unit_price FROM order_items WHERE order_id = $1", r.Ms(6, 14))
		}
		r.Compute("encode response", r.Ms(2, 6)).Scope = "orders-api"
		r.Gap(r.Ms(0.2, 0.6))
	})

	order := api.Endpoint("GET", "/api/orders/{id}")
	order.Traffic(seed.Daily(120), seed.Diurnal(), seed.Over(window))
	order.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.1))
		r.DB("SELECT id, user_id, status, total_cents, created_at FROM orders WHERE id = $1", r.Ms(2, 5))
		r.DB("SELECT id, sku, qty, unit_price FROM order_items WHERE order_id = $1", r.Ms(3, 7))
		r.Compute("encode response", r.Ms(0.5, 1.5)).Scope = "orders-api"
	})

	s.Background(api, seed.TypicalTraffic(10))
	api.Host().CPU(seed.Flat(0.22, 0.31))
	api.Host().Memory(seed.Flat(0.48, 0.55))

	s.Expose("slowRequest", orders.Slowest(24*time.Hour))
	s.Expose("p50Ms24h", func() any { return orders.Percentile(0.5, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p95Ms24h", func() any { return orders.Percentile(0.95, 24*time.Hour).Seconds() * 1000 })
	s.Expose("p95MsFirstWeek", func() any {
		return orders.PercentileBetween(0.95, s.Ago(window), s.Ago(window-7*24*time.Hour)).Seconds() * 1000
	})
	return nil
}
