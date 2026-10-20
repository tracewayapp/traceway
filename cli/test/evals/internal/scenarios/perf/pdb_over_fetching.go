package perf

import (
	"math"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/db-over-fetching", pdbSeedOverFetching) }

// pdbSeedOverFetching: the category grid loads its 48 products with SELECT *.
// Release 4.8.0 added products.supplier_payload (the supplier's raw feed,
// about 120KB per product), and the importer filled it in over the next day
// and a half. The grid query returns the same 48 rows as before but now
// drags every payload across the wire and through the driver's JSON parsing,
// so its span grows from ~12ms to ~400ms and the Node heap and host memory
// climb with it. The search endpoint reads the same table, 48 rows, with an
// explicit column list and stays fast. Distractor: JSON encoding (a
// teammate's guess): the response is 48 small cards, encoded in a couple of
// milliseconds after the query, and CPU rises only modestly.
func pdbSeedOverFetching(s *seed.Scenario) error {
	window := 14 * 24 * time.Hour
	deployAt := s.Ago(10*24*time.Hour + 3*time.Hour)
	backfill := 36 * time.Hour
	filled := func(t time.Time) float64 {
		if t.Before(deployAt) {
			return 0
		}
		return math.Min(1, float64(t.Sub(deployAt))/float64(backfill))
	}
	p := s.Project("shopfront", seed.OTel)
	api := p.Service("catalog-api", seed.Node("express", "pg")).
		Versions("4.7.2", seed.Deploy{At: deployAt, Version: "4.8.0"})

	grid := api.Endpoint("GET", "/api/categories/:categoryId/products")
	grid.Traffic(seed.Daily(260), seed.Diurnal(), seed.Over(window))
	grid.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.12, Max: 12})
	grid.Handler(func(r *seed.Req) {
		f := filled(r.Start())
		r.Gap(r.Ms(0.4, 1.1)) // session + query middleware
		r.DB("SELECT id, name, slug FROM categories WHERE id = $1", r.Ms(0.5, 1.4), seed.Rows(1))
		width := r.Ms(9, 16) + time.Duration(f*float64(r.Ms(300, 470)))
		r.DB("SELECT * FROM products WHERE category_id = $1 AND is_active = true ORDER BY sort_rank DESC LIMIT 48", width, seed.Rows(48))
		r.Gap(r.Ms(1.2, 2.8)) // map 48 rows to cards, res.json
	})

	product := api.Endpoint("GET", "/api/products/:id")
	product.Traffic(seed.Daily(180), seed.Diurnal(), seed.Over(window))
	product.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.03, Max: 4})
	product.Handler(func(r *seed.Req) {
		f := filled(r.Start())
		r.Gap(r.Ms(0.4, 1.1))
		r.DB("SELECT * FROM products WHERE id = $1", r.Ms(0.8, 2.0)+time.Duration(f*float64(r.Ms(6, 11))), seed.Rows(1))
		r.DB("SELECT id, url, alt, position FROM product_images WHERE product_id = $1 ORDER BY position", r.Ms(0.6, 1.6), seed.Rows(6))
		r.Gap(r.Ms(0.4, 1.0))
	})

	search := api.Endpoint("GET", "/api/search")
	search.Traffic(seed.Daily(150), seed.Diurnal(), seed.Over(window))
	search.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.04, Max: 4})
	search.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.1))
		r.DB("SELECT id, name, slug, price_cents, currency, thumbnail_url FROM products WHERE is_active = true AND search_vector @@ plainto_tsquery('english', $1) ORDER BY ts_rank(search_vector, plainto_tsquery('english', $1)) DESC LIMIT 48",
			r.LogNormal(11*time.Millisecond, 26*time.Millisecond), seed.Rows(48))
		r.Gap(r.Ms(0.8, 1.8))
	})

	pdbHealthy(api, window,
		pdbRoute{method: "GET", route: "/api/products/:id/price", perDay: 60, queries: []string{
			"SELECT id, price_cents, currency, compare_at_cents FROM products WHERE id = $1",
		}, dbMedianMs: 0.9},
		pdbRoute{method: "GET", route: "/api/brands", perDay: 25, queries: []string{
			"SELECT id, name, slug, logo_url FROM brands ORDER BY name",
		}, dbMedianMs: 1.5},
		pdbRoute{method: "GET", route: "/api/products/:id/reviews", perDay: 40, queries: []string{
			"SELECT id, rating, title, body, author_name, created_at FROM reviews WHERE product_id = $1 ORDER BY created_at DESC LIMIT 20",
		}, dbMedianMs: 2},
	)

	busy := func(t time.Time) float64 { return 0.04 * filled(t) }
	api.Host().CPU(seed.Plus(seed.Flat(0.21, 0.29), busy))
	api.Host().Memory(seed.Plus(seed.Flat(0.45, 0.49), func(t time.Time) float64 { return 0.19 * filled(t) }))
	heapMB := seed.Plus(seed.Sawtooth(45*time.Minute, 150, 210), func(t time.Time) float64 { return filled(t) * 340 })
	api.Gauge("v8js.memory.heap.used", "By", map[string]any{"v8js.heap.space.name": "old_space"},
		func(t time.Time) float64 { return heapMB(t) * 1024 * 1024 })

	s.Expose("slowRequest", pdbSlowest(grid, s.Ago(24*time.Hour), nil))
	// The newest grid request keeps its trace and access log, so a recent
	// one is there at any hour; overnight the sample alone can leave the
	// last six hours without one.
	s.Expose("latestRequest", grid.Pick(func(*seed.Request) bool { return true }))
	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	s.Expose("gridP50Ms24h", pdbMs(grid, 0.5, s.Ago(24*time.Hour), s.Now))
	s.Expose("gridP50MsBeforeDeploy", pdbMs(grid, 0.5, s.Ago(window), deployAt))
	s.Expose("searchP50Ms24h", pdbMs(search, 0.5, s.Ago(24*time.Hour), s.Now))
	return nil
}
