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

	// Sanctum's guard on every request: the token by id, its user, and the
	// last_used_at write, each through Eloquent.
	const tokenModel, userModel = "Laravel\\Sanctum\\PersonalAccessToken", "App\\Models\\User"
	const tokenSQL = "select * from `personal_access_tokens` where `personal_access_tokens`.`id` = ? limit 1"
	const userSQL = "select * from `users` where `users`.`id` = ? limit 1"
	auth := func(r *seed.Req) {
		r.Gap(r.Ms(2.5, 5.5)) // framework boot and middleware
		pdbEloquent(r, tokenModel, "personal_access_tokens", "find", "Illuminate\\Database\\Eloquent\\Builder::find", "", func(r *seed.Req) {
			pdbEloquent(r, tokenModel, "personal_access_tokens", "get", "Illuminate\\Database\\Eloquent\\Builder::getModels", tokenSQL, func(r *seed.Req) {
				pdbLaravelSQL(r, tokenSQL, r.Ms(0.5, 1.3))
			})
		})
		pdbEloquent(r, userModel, "users", "get", "Illuminate\\Database\\Eloquent\\Builder::getModels", userSQL, func(r *seed.Req) {
			pdbLaravelSQL(r, userSQL, r.Ms(0.4, 1.1))
		})
		pdbEloquent(r, tokenModel, "personal_access_tokens", "update", "Illuminate\\Database\\Eloquent\\Model::performUpdate", "", func(r *seed.Req) {
			pdbLaravelSQL(r, "update `personal_access_tokens` set `last_used_at` = ?, `personal_access_tokens`.`updated_at` = ? where `id` = ?", r.Ms(0.6, 1.6))
		})
	}

	report := api.Endpoint("GET", "/api/stores/{store}/reports/revenue")
	report.Traffic(seed.Daily(200), seed.Diurnal(), seed.Over(window))
	report.Detail(seed.DetailPolicy{Recent: 18 * time.Hour, Rate: 0.06, Max: 12})
	report.Handler(func(r *seed.Req) {
		auth(r)
		pdbLaravelSQL(r, "select `stores`.`id`, `stores`.`name`, `stores`.`currency` from `stores` where `stores`.`id` = ? and `stores`.`merchant_id` = ? limit 1", r.Ms(0.5, 1.2))
		pdbLaravelSQL(r, pdbRevenueSQL, r.Ms(1650, 1990))
		r.Gap(r.Ms(3, 7)) // collection mapping and the JSON response
	})

	orders := api.Endpoint("GET", "/api/stores/{store}/orders")
	orders.Traffic(seed.Daily(220), seed.Diurnal(), seed.Over(window))
	orders.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.02, Max: 6})
	orders.Handler(func(r *seed.Req) {
		auth(r)
		pdbLaravelSQL(r, "select count(*) as aggregate from `orders` where `orders`.`store_id` = ?", r.Ms(1.5, 4))
		pdbLaravelSQL(r, "select `orders`.`id`, `orders`.`number`, `orders`.`status`, `orders`.`total`, `orders`.`customer_id`, `orders`.`created_at` from `orders` where `orders`.`store_id` = ? order by `orders`.`created_at` desc limit 25 offset 0", r.Ms(1.2, 3.5))
		r.Gap(r.Ms(1.5, 4))
	})

	for _, rt := range []pdbRoute{
		{method: "GET", route: "/api/user", perDay: 90},
		{method: "GET", route: "/api/stores/{store}/orders/{order}", perDay: 80, queries: []string{
			"select `orders`.`id`, `orders`.`number`, `orders`.`status`, `orders`.`total`, `orders`.`customer_id`, `orders`.`created_at` from `orders` where `orders`.`id` = ? and `orders`.`store_id` = ? limit 1",
			"select `order_items`.`id`, `order_items`.`product_id`, `order_items`.`quantity`, `order_items`.`unit_price` from `order_items` where `order_items`.`order_id` = ?",
		}, dbMedianMs: 1.2},
		{method: "GET", route: "/api/stores/{store}/products", perDay: 60, queries: []string{
			"select `products`.`id`, `products`.`sku`, `products`.`name`, `products`.`price`, `products`.`category_id` from `products` where `products`.`store_id` = ? order by `products`.`name` asc limit 50 offset 0",
		}, dbMedianMs: 2.5},
		{method: "PUT", route: "/api/stores/{store}/products/{product}", perDay: 15, queries: []string{
			"update `products` set `name` = ?, `price` = ?, `updated_at` = ? where `id` = ?",
		}, dbMedianMs: 1.6},
	} {
		rt := rt
		e := api.Endpoint(rt.method, rt.route)
		e.Traffic(seed.Daily(rt.perDay), seed.Diurnal(), seed.Over(window))
		e.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 4})
		e.Handler(func(r *seed.Req) {
			auth(r)
			for _, q := range rt.queries {
				m := pdbDur(rt.dbMedianMs)
				pdbLaravelSQL(r, q, r.LogNormal(m, m*5))
			}
			r.Gap(r.Ms(0.6, 2.4))
		})
	}
	api.Host().CPU(seed.DiurnalSeries(seed.Flat(0.22, 0.27), 0.3))
	api.Host().Memory(seed.Flat(0.52, 0.58))

	s.Expose("slowRequest", pdbSlowest(report, s.Ago(18*time.Hour), nil))
	s.Expose("p50Ms24h", pdbMs(report, 0.5, s.Ago(24*time.Hour), s.Now))
	s.Expose("p95Ms24h", pdbMs(report, 0.95, s.Ago(24*time.Hour), s.Now))
	s.Expose("p50MsFirstDays", pdbMs(report, 0.5, s.Ago(window), s.Ago(window-3*24*time.Hour)))
	return nil
}

const pdbRevenueSQL = "select date(orders.created_at) as day, products.category_id, count(distinct orders.customer_id) as buyers, sum(order_items.quantity * order_items.unit_price) as revenue from `orders` inner join `order_items` on `order_items`.`order_id` = `orders`.`id` inner join `products` on `products`.`id` = `order_items`.`product_id` where `orders`.`store_id` = ? and `orders`.`status` = ? and `orders`.`created_at` >= ? group by `day`, `products`.`category_id` order by `day` asc"

// pdbEloquent records the INTERNAL span opentelemetry-auto-laravel wraps
// around an Eloquent operation ("<Model>::find|get|update").
func pdbEloquent(r *seed.Req, model, table, op, hooked, statement string, f func(r *seed.Req)) {
	attrs := []seed.KV{seed.A("code.function.name", hooked), seed.A("laravel.eloquent.model", model),
		seed.A("laravel.eloquent.table", table), seed.A("laravel.eloquent.operation", op)}
	if statement != "" {
		attrs = append(attrs, seed.A("db.statement", statement))
	}
	sp := r.Span(model+"::"+op, func(r *seed.Req) {
		r.Gap(r.Ms(0.02, 0.08))
		f(r)
		r.Gap(r.Ms(0.05, 0.25)) // hydrate the model, fire its events
	}, attrs...)
	sp.Scope = pdbLaravelScope
}

const pdbLaravelScope = "io.opentelemetry.contrib.php.laravel"

// pdbLaravelSQL records a query the way opentelemetry-auto-laravel's
// QueryWatcher reports it: "sql <OPERATION>" under the Laravel scope, with
// the grammar's SQL text and no row count or peer address.
func pdbLaravelSQL(r *seed.Req, query string, d time.Duration) *seed.Span {
	sp := r.DB(query, d)
	op, _ := sp.Attrs["db.operation.name"].(string)
	sp.Name = "sql " + op
	sp.Scope = pdbLaravelScope
	for _, k := range []string{"db.collection.name", "server.address", "server.port"} {
		delete(sp.Attrs, k)
	}
	return sp
}
