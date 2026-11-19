package perf

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/db-missing-query-cache", pdbSeedMissingQueryCache) }

const pdbNestScope = "@opentelemetry/instrumentation-nestjs-core"

var pdbCategorySlugs = []string{"running-shoes", "rain-jackets", "base-layers", "hiking-boots", "backpacks", "tents", "sleeping-bags", "headlamps", "trail-socks", "water-bottles", "camp-kitchen", "climbing-ropes"}

// pdbSeedMissingQueryCache: a global NestJS guard loads every feature flag
// from Postgres on every request, the same parameterless query returning the
// same 214 rows, before any handler runs. It costs ~15-30ms per request and
// is most of the time of the cheap endpoints (GET /api/me reads one customer
// row in ~1.5ms). Distractor: no cache / a low hit ratio. Product data is
// already behind Redis and the app's cache.hit_ratio gauge sits at 96-99%;
// the uncached thing is one specific query.
func pdbSeedMissingQueryCache(s *seed.Scenario) error {
	window := 3 * 24 * time.Hour
	p := s.Project("storefront-web", seed.OTel)
	api := p.Service("storefront-api", seed.Node("nestjs", "pg")).Versions("7.4.2")

	const flagsSQL = "SELECT key, enabled, rollout_percent, rules FROM feature_flags WHERE archived_at IS NULL ORDER BY key"
	guard := func(r *seed.Req) {
		r.DB(flagsSQL, r.LogNormal(19*time.Millisecond, 34*time.Millisecond), seed.Rows(214))
	}
	cached := func(r *seed.Req, key, missSQL string, missMs [2]float64, rows int) {
		r.Cache("get", key, r.Ms(0.3, 0.9))
		if r.Chance(0.02) {
			r.DB(missSQL, r.Ms(missMs[0], missMs[1]), seed.Rows(rows))
			r.Cache("set", key, r.Ms(0.3, 0.8))
		}
	}
	type route struct {
		method, path, controller, handler string
		perDay                            int
		body                              func(r *seed.Req)
	}
	routes := []route{
		{"GET", "/api/me", "MeController", "getMe", 900, func(r *seed.Req) {
			r.DB("SELECT id, email, first_name, last_name, loyalty_tier FROM customers WHERE id = $1", r.Ms(0.8, 2.2), seed.Rows(1))
		}},
		{"GET", "/api/products/:id", "ProductsController", "findOne", 1100, func(r *seed.Req) {
			id := 10000 + r.IntN(50000)
			r.RootAttr("url.path", fmt.Sprintf("/api/products/%d", id))
			cached(r, fmt.Sprintf("product:%d", id),
				"SELECT id, slug, name, price_cents, currency, images, attributes FROM products WHERE id = $1", [2]float64{3, 7}, 1)
		}},
		{"GET", "/api/categories/:slug/products", "ProductsController", "byCategory", 700, func(r *seed.Req) {
			slug := pdbCategorySlugs[r.IntN(len(pdbCategorySlugs))]
			r.RootAttr("url.path", "/api/categories/"+slug+"/products")
			cached(r, "category:"+slug+":page:1",
				"SELECT p.id, p.slug, p.name, p.price_cents, p.currency, p.thumbnail_url FROM products p JOIN categories c ON c.id = p.category_id WHERE c.slug = $1 ORDER BY p.rank LIMIT 48", [2]float64{6, 14}, 48)
		}},
		{"GET", "/api/cart", "CartController", "get", 650, func(r *seed.Req) {
			r.DB("SELECT l.sku, l.qty, l.unit_price_cents FROM cart_lines l WHERE l.cart_id = $1", r.Ms(1.2, 3.2), seed.Rows(1+r.IntN(5)))
		}},
		{"POST", "/api/cart/lines", "CartController", "addLine", 260, func(r *seed.Req) {
			r.DB("INSERT INTO cart_lines (cart_id, sku, qty, unit_price_cents) VALUES ($1, $2, $3, $4) ON CONFLICT (cart_id, sku) DO UPDATE SET qty = cart_lines.qty + EXCLUDED.qty", r.Ms(1.5, 3.8))
		}},
	}
	var me, category *seed.Endpoint
	for _, rt := range routes {
		rt := rt
		e := api.Endpoint(rt.method, rt.path)
		e.Traffic(seed.Daily(rt.perDay), seed.Diurnal(), seed.Over(window))
		e.Detail(seed.DetailPolicy{Recent: 8 * time.Hour, Rate: 0.03, Max: 6})
		e.Handler(func(r *seed.Req) {
			r.Gap(r.Ms(0.3, 0.9)) // session middleware
			// The request-context span covers the guards; the handler span
			// is the controller method itself.
			reqCtx := r.Span(rt.controller+"."+rt.handler, func(r *seed.Req) {
				guard(r)
				h := r.Span(rt.handler, func(r *seed.Req) {
					rt.body(r)
					r.Gap(r.Ms(0.3, 1.2)) // serialize
				}, seed.A("nestjs.type", "handler"), seed.A("nestjs.callback", rt.handler))
				h.Scope = pdbNestScope
			}, seed.A("nestjs.type", "request_context"), seed.A("nestjs.controller", rt.controller), seed.A("nestjs.callback", rt.handler), seed.A("http.route", rt.path))
			reqCtx.Scope = pdbNestScope
			r.Gap(r.Ms(0.1, 0.4))
		})
		switch rt.path {
		case "/api/me":
			me = e
		case "/api/categories/:slug/products":
			category = e
		}
	}

	// Gauge adds +-1.5% jitter, so this reads about 96-99.9% and never above 1.
	api.Gauge("cache.hit_ratio", "1", map[string]any{"cache.name": "product"}, seed.Flat(0.972, 0.984))
	api.Host().CPU(seed.DiurnalSeries(seed.Flat(0.24, 0.3), 0.25))
	api.Host().Memory(seed.Flat(0.47, 0.52))

	day := s.Ago(24 * time.Hour)
	s.Expose("slowRequest", pdbSlowest(me, day, nil))
	// The newest request of these two endpoints keeps its trace and access
	// log, so a recent one is there at any hour; overnight the 3% sample
	// alone can leave the last eight hours without one.
	s.Expose("latestMeRequest", me.Pick(func(*seed.Request) bool { return true }))
	s.Expose("latestCategoryRequest", category.Pick(func(*seed.Request) bool { return true }))
	s.Expose("meP50Ms24h", pdbMs(me, 0.5, day, s.Now))
	s.Expose("meP95Ms24h", pdbMs(me, 0.95, day, s.Now))
	return nil
}
