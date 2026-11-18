package perf

import (
	"math"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/infra-cpu-saturation", papSeedCPUSaturation) }

// papSeedCPUSaturation: 16-40h ago (14:20 UTC) a campaign tripled commerce-api's traffic
// (ramping over 40 minutes) and it has stayed there. Both instances went
// from ~25% to ~95% CPU and every endpoint got several times slower at once:
// each span is stretched and run-queue gaps appear, with no single long pole.
// Nothing was deployed. Distractor: a code regression; but service.version
// never changed, no commit landed, and the slowdown follows the request rate
// on every endpoint alike.
func papSeedCPUSaturation(s *seed.Scenario) error {
	p := s.Project("commerce", seed.OTel)
	api := p.Service("commerce-api", seed.Node("express", "pg")).Versions("4.18.2").
		Instances("commerce-api-7f6d9c4b8-2xkqv", "commerce-api-7f6d9c4b8-9jw4t")
	window := 7 * 24 * time.Hour
	// The campaign went live at 14:20 UTC, 16-40 hours before the run.
	onset := s.Now.UTC().Truncate(24 * time.Hour).Add(14*time.Hour + 20*time.Minute)
	for onset.After(s.Ago(16 * time.Hour)) {
		onset = onset.Add(-24 * time.Hour)
	}
	ramp := 40 * time.Minute

	load := func(t time.Time) float64 { // traffic multiple of the usual rate
		switch {
		case t.Before(onset):
			return 1
		case t.Before(onset.Add(ramp)):
			return 1 + 2*float64(t.Sub(onset))/float64(ramp)
		default:
			return 3
		}
	}
	diurnal := func(t time.Time) float64 {
		h := float64(t.Hour()) + float64(t.Minute())/60
		return 1 + 0.25*math.Cos((h-14)/24*2*math.Pi)
	}
	// Utilization follows the request rate; past ~70% the run queue grows
	// and every request is stretched.
	util := func(t time.Time) float64 { return math.Min(0.97, 0.27*load(t)*diurnal(t)) }
	stretch := func(t time.Time) float64 {
		u := util(t)
		if u <= 0.45 {
			return 1
		}
		return 1 + 2.6*math.Pow((u-0.45)/0.52, 1.5)
	}
	traffic := func(e *seed.Endpoint, salt string, perDay int) {
		e.Traffic(papJitter(salt, 0.06), seed.RateFunc(load), seed.RateFunc(diurnal), seed.Daily(perDay), seed.Over(window))
		e.Detail(seed.DetailPolicy{Recent: 24 * time.Hour, Rate: 0.02, Max: 12})
	}
	// cpu is in-process work (no span: Express is instrumented only at the
	// edges), stretched fully; db is a query, of which only the client-side
	// half (driver, event loop) is stretched.
	cpu := func(r *seed.Req, d time.Duration) time.Duration {
		return time.Duration(float64(d) * stretch(r.Start()))
	}
	db := func(r *seed.Req, d time.Duration) time.Duration {
		return time.Duration(float64(d) * (1 + 0.5*(stretch(r.Start())-1)))
	}
	queue := func(r *seed.Req) {
		r.Gap(cpu(r, r.Ms(0.3, 0.9)) + time.Duration((stretch(r.Start())-1)*float64(r.Ms(2, 9))))
	}

	products := api.Endpoint("GET", "/api/products")
	traffic(products, "products", 260)
	products.Handler(func(r *seed.Req) {
		queue(r)
		r.DB("SELECT id, sku, name, price_cents, thumbnail_url FROM products WHERE category_id = $1 AND visible ORDER BY rank LIMIT 48", db(r, r.Ms(6, 14)), seed.Rows(48))
		r.Gap(cpu(r, r.Ms(4, 9))) // serialize products
	})
	product := api.Endpoint("GET", "/api/products/:id")
	traffic(product, "product", 320)
	product.Handler(func(r *seed.Req) {
		queue(r)
		r.DB("SELECT id, sku, name, description, price_cents, stock FROM products WHERE id = $1", db(r, r.Ms(1.5, 4)))
		r.DB("SELECT url, alt FROM product_images WHERE product_id = $1 ORDER BY position", db(r, r.Ms(1.5, 4)))
		r.Gap(cpu(r, r.Ms(3, 7))) // render product
	})
	search := api.Endpoint("GET", "/api/search")
	traffic(search, "search", 240)
	search.Handler(func(r *seed.Req) {
		queue(r)
		r.HTTP("POST", "http://search.internal:9200/products/_search", db(r, r.Ms(12, 30)), 200)
		r.Gap(cpu(r, r.Ms(6, 14))) // hydrate results
	})
	cart := api.Endpoint("GET", "/api/cart")
	traffic(cart, "cart", 220)
	cart.Handler(func(r *seed.Req) {
		queue(r)
		r.DB("SELECT p.id, p.name, p.price_cents, ci.qty FROM cart_items ci JOIN products p ON p.id = ci.product_id WHERE ci.cart_id = $1", db(r, r.Ms(2, 5)))
		r.Gap(cpu(r, r.Ms(2, 5))) // price cart
	})
	add := api.Endpoint("POST", "/api/cart/items")
	traffic(add, "add", 160)
	add.Handler(func(r *seed.Req) {
		queue(r)
		r.Gap(cpu(r, r.Ms(0.6, 1.5))) // validate body
		r.DB("INSERT INTO cart_items (cart_id, product_id, qty) VALUES ($1, $2, $3) ON CONFLICT (cart_id, product_id) DO UPDATE SET qty = cart_items.qty + EXCLUDED.qty", db(r, r.Ms(2, 5)))
		r.Status(201)
	})
	checkout := api.Endpoint("POST", "/api/checkout")
	traffic(checkout, "checkout", 70)
	checkout.Handler(func(r *seed.Req) {
		queue(r)
		r.DB("BEGIN", db(r, r.Ms(0.2, 0.6)))
		r.DB("SELECT p.id, p.price_cents, p.stock, ci.qty FROM cart_items ci JOIN products p ON p.id = ci.product_id WHERE ci.cart_id = $1 FOR UPDATE", db(r, r.Ms(3, 7)))
		r.Gap(cpu(r, r.Ms(4, 9))) // price order
		r.HTTP("POST", "https://api.payments.example/v1/payment_intents", r.LogNormal(180*time.Millisecond, 420*time.Millisecond), 200)
		r.DB("INSERT INTO orders (id, user_id, total_cents, status, created_at) VALUES ($1, $2, $3, 'paid', now())", db(r, r.Ms(2, 5)))
		r.DB("COMMIT", db(r, r.Ms(0.6, 1.8)))
		r.Status(201)
	})
	recs := api.Endpoint("GET", "/api/recommendations")
	traffic(recs, "recs", 200)
	recs.Handler(func(r *seed.Req) {
		queue(r)
		r.DB("SELECT product_id, score FROM recommendations WHERE user_id = $1 ORDER BY score DESC LIMIT 12", db(r, r.Ms(3, 8)))
		r.Gap(cpu(r, r.Ms(5, 12))) // rank recommendations
	})
	cats := api.Endpoint("GET", "/api/categories")
	traffic(cats, "cats", 130)
	cats.Handler(func(r *seed.Req) {
		queue(r)
		r.Cache("GET", "categories:tree", db(r, r.Ms(0.3, 0.9)))
		r.Gap(cpu(r, r.Ms(1, 3))) // serialize categories
	})

	for _, h := range api.Hosts {
		api.HostNamed(h).CPU(seed.Plus(seed.Flat(-0.02, 0.02), util))
		api.HostNamed(h).Memory(seed.Plus(seed.Flat(0.42, 0.46), func(t time.Time) float64 { return 0.04 * (load(t) - 1) }))
	}

	rfc := func(t time.Time) string { return t.UTC().Format(time.RFC3339) }
	s.Expose("onsetAt", rfc(onset))
	s.Expose("beforeFrom", rfc(onset.Add(-24*time.Hour+time.Hour)))
	s.Expose("beforeTo", rfc(onset.Add(-24*time.Hour+7*time.Hour)))
	s.Expose("afterFrom", rfc(onset.Add(time.Hour)))
	s.Expose("afterTo", rfc(onset.Add(7*time.Hour)))
	var slow *seed.Request
	for _, rq := range product.Requests() {
		if rq.Start.After(onset.Add(time.Hour)) && rq.Start.Before(onset.Add(7*time.Hour)) && (slow == nil || rq.Duration() > slow.Duration()) {
			slow = rq
		}
	}
	slow.KeepDetail()
	s.Expose("slowRequest", slow.Ref())
	s.Expose("earlierRequest", product.Pick(func(rq *seed.Request) bool {
		return rq.Start.After(onset.Add(-23*time.Hour)) && rq.Start.Before(onset.Add(-17*time.Hour)) && rq.Duration() > 10*time.Millisecond
	}))
	s.Expose("p95ProductBefore", func() any {
		return product.PercentileBetween(0.95, onset.Add(-23*time.Hour), onset.Add(-17*time.Hour)).Seconds() * 1000
	})
	s.Expose("p95ProductAfter", func() any {
		return product.PercentileBetween(0.95, onset.Add(time.Hour), onset.Add(7*time.Hour)).Seconds() * 1000
	})
	return nil
}
