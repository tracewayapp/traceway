package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/db-expensive-join", pdbSeedExpensiveJoin) }

// pdbSeedExpensiveJoin: the merchant revenue report joins 90 days of a
// store's orders to their items and products and aggregates by day and
// category on every request. That one query takes ~1.8s every time, at the
// quiet end of the night and at the daily peak alike, and has done so for
// the whole window (no growth, no deploy). Distractor: saturation, which ops
// suspects. CPU moves gently with the daily traffic and stays under 40%, and
// every other endpoint is fast at the same moments.
func pdbSeedExpensiveJoin(s *seed.Scenario) error {
	window := 14 * 24 * time.Hour
	p := s.Project("merchant-hub", seed.OTel)
	api := p.Service("merchant-api", seed.PHP("laravel")).Versions("2.41.0")

	auth := func(r *seed.Req) {
		r.Gap(r.Ms(2.5, 5.5)) // framework boot, session and auth middleware
		r.DB("select * from personal_access_tokens where token = ? limit 1", r.Ms(0.5, 1.3), seed.Rows(1))
	}

	report := api.Endpoint("GET", "/api/stores/{store}/reports/revenue")
	report.Traffic(seed.Daily(200), seed.Diurnal(), seed.Over(window))
	report.Detail(seed.DetailPolicy{Recent: 18 * time.Hour, Rate: 0.06, Max: 12})
	report.Handler(func(r *seed.Req) {
		auth(r)
		r.DB("select stores.id, stores.name, stores.currency from stores where stores.id = ? and stores.merchant_id = ? limit 1", r.Ms(0.5, 1.2), seed.Rows(1))
		r.DB("select date(orders.created_at) as day, products.category_id, count(distinct orders.customer_id) as buyers, sum(order_items.quantity * order_items.unit_price) as revenue from orders inner join order_items on order_items.order_id = orders.id inner join products on products.id = order_items.product_id where orders.store_id = ? and orders.status = ? and orders.created_at >= ? group by day, products.category_id order by day asc",
			r.Ms(1650, 1990), seed.Rows(700+r.IntN(260)))
		r.Gap(r.Ms(3, 7)) // collection mapping and the JSON response
	})

	orders := api.Endpoint("GET", "/api/stores/{store}/orders")
	orders.Traffic(seed.Daily(220), seed.Diurnal(), seed.Over(window))
	orders.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.02, Max: 6})
	orders.Handler(func(r *seed.Req) {
		auth(r)
		r.DB("select count(*) as aggregate from orders where orders.store_id = ?", r.Ms(1.5, 4), seed.Rows(1))
		r.DB("select orders.id, orders.number, orders.status, orders.total, orders.customer_id, orders.created_at from orders where orders.store_id = ? order by orders.created_at desc limit 25 offset 0", r.Ms(1.2, 3.5), seed.Rows(25))
		r.Gap(r.Ms(1.5, 4))
	})

	pdbHealthy(api, window,
		pdbRoute{method: "GET", route: "/api/user", perDay: 90, queries: []string{
			"select * from personal_access_tokens where token = ? limit 1",
			"select * from users where users.id = ? limit 1",
		}, dbMedianMs: 0.8},
		pdbRoute{method: "GET", route: "/api/stores/{store}/orders/{order}", perDay: 80, queries: []string{
			"select orders.id, orders.number, orders.status, orders.total, orders.customer_id, orders.created_at from orders where orders.id = ? and orders.store_id = ? limit 1",
			"select order_items.id, order_items.product_id, order_items.quantity, order_items.unit_price from order_items where order_items.order_id = ?",
		}, dbMedianMs: 1.2},
		pdbRoute{method: "GET", route: "/api/stores/{store}/products", perDay: 60, queries: []string{
			"select products.id, products.sku, products.name, products.price, products.category_id from products where products.store_id = ? order by products.name asc limit 50 offset 0",
		}, dbMedianMs: 2.5},
		pdbRoute{method: "PUT", route: "/api/stores/{store}/products/{product}", perDay: 15, queries: []string{
			"update products set name = ?, price = ?, updated_at = ? where id = ?",
		}, dbMedianMs: 1.6},
	)
	api.Host().CPU(seed.DiurnalSeries(seed.Flat(0.22, 0.27), 0.3))
	api.Host().Memory(seed.Flat(0.52, 0.58))

	s.Expose("slowRequest", pdbSlowest(report, s.Ago(18*time.Hour), nil))
	s.Expose("p50Ms24h", pdbMs(report, 0.5, s.Ago(24*time.Hour), s.Now))
	s.Expose("p95Ms24h", pdbMs(report, 0.95, s.Ago(24*time.Hour), s.Now))
	s.Expose("p50MsFirstDays", pdbMs(report, 0.5, s.Ago(window), s.Ago(window-3*24*time.Hour)))
	return nil
}
