package perf

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/db-pool-exhaustion", pdbSeedPoolExhaustion) }

// pdbPoolTimeout is the SQLAlchemy QueuePool message for pool_size=10,
// max_overflow=5, pool_timeout=30.
const pdbPoolTimeout = "QueuePool limit of size 10 overflow 5 reached, connection timed out, timeout 30.00 (Background on this error at: https://sqlalche.me/e/20/3o7r)"

// pdbCartItems is select(CartItem) by cart, as SQLAlchemy renders it for psycopg.
const pdbCartItems = "SELECT cart_items.id, cart_items.cart_id, cart_items.sku, cart_items.qty, cart_items.unit_price_cents FROM cart_items WHERE cart_items.cart_id = %(cart_id_1)s::UUID"

// pdbStockQuery is the stock check; in_() expands to one parameter per SKU.
func pdbStockQuery(skus int) string {
	params := make([]string, skus)
	for i := range params {
		params[i] = fmt.Sprintf("%%(sku_1_%d)s::VARCHAR", i+1)
	}
	return "SELECT inventory.sku, inventory.available FROM inventory WHERE inventory.sku IN (" + strings.Join(params, ", ") + ")"
}

// pdbSeedPoolExhaustion: checkout holds its pooled connection for the whole
// request, including the payment provider call. At the lunch and evening
// rushes the 15 connections (pool_size 10 + max_overflow 5) are all checked
// out, and requests wait for one before their first query: a gap with no
// span before the first DB span, seconds long, only at the peaks. A few wait
// the full pool_timeout and fail with sqlalchemy.exc.TimeoutError. The cart
// endpoint shares the pool and waits the same way although its only query
// takes 3ms; the shipping options, which need no connection, never do. The
// pool's own metric shows 15 used and 0 idle at exactly those hours.
// Distractors: a slow database (the inventory query takes 40-60ms at every
// hour) and the payment provider (150-260ms at every hour, peaks included).
func pdbSeedPoolExhaustion(s *seed.Scenario) error {
	window := 3 * 24 * time.Hour
	p := s.Project("marketplace", seed.OTel)
	api := p.Service("checkout-api", seed.Python("fastapi", "sqlalchemy")).Versions("5.3.1")

	// bump is 0 off-peak and 1 on the plateau of the lunch (12:00-13:00)
	// and evening (19:00-20:30) rushes, with 25-minute ramps.
	type rush struct{ from, to float64 }
	rushes := []rush{{12, 13}, {19, 20.5}}
	const ramp = 25.0 / 60
	bump := func(t time.Time) float64 {
		h := float64(t.Hour()) + float64(t.Minute())/60 + float64(t.Second())/3600
		best := 0.0
		for _, r := range rushes {
			var v float64
			switch {
			case h >= r.from && h <= r.to:
				v = 1
			case h > r.from-ramp && h < r.from:
				v = (h - (r.from - ramp)) / ramp
			case h > r.to && h < r.to+ramp:
				v = (r.to + ramp - h) / ramp
			}
			best = math.Max(best, v)
		}
		return best
	}
	load := func(t time.Time) float64 { return 1 + 2.4*bump(t) }
	// over is how far demand for connections exceeds the 15 the pool has,
	// 0..1; it is non-zero only near the plateau of a rush.
	over := func(t time.Time) float64 { return math.Max(0, (load(t)-2.7)/0.7) }

	// waitFor is the time a request spends in QueuePool.get before its first
	// query; ok is false when it waited the whole pool_timeout.
	waitFor := func(r *seed.Req) (time.Duration, bool) {
		o := over(r.Start())
		if o == 0 || !r.Chance(0.25+0.65*o) {
			return 0, true
		}
		w := time.Duration(o * float64(r.LogNormal(3200*time.Millisecond, 26*time.Second)))
		if w >= 30*time.Second {
			return 30*time.Second + r.Ms(1, 4), false
		}
		return w, true
	}
	poolTimeout := func(r *seed.Req, frame string) {
		r.Error(seed.Exc{Type: "sqlalchemy.exc.TimeoutError", Message: pdbPoolTimeout, Escaped: true, Stack: "Traceback (most recent call last):\n" +
			"  File \"/usr/local/lib/python3.12/site-packages/fastapi/routing.py\", line 219, in run_endpoint_function\n    return await run_in_threadpool(dependant.call, **values)\n" +
			"  File \"/usr/local/lib/python3.12/site-packages/starlette/concurrency.py\", line 38, in run_in_threadpool\n    return await anyio.to_thread.run_sync(func)\n" +
			frame +
			"  File \"/usr/local/lib/python3.12/site-packages/sqlalchemy/orm/session.py\", line 2365, in execute\n    return self._execute_internal(\n" +
			"  File \"/usr/local/lib/python3.12/site-packages/sqlalchemy/orm/session.py\", line 2241, in _execute_internal\n    conn = self._connection_for_bind(bind)\n" +
			"  File \"/usr/local/lib/python3.12/site-packages/sqlalchemy/engine/base.py\", line 3297, in raw_connection\n    return self.pool.connect()\n" +
			"  File \"/usr/local/lib/python3.12/site-packages/sqlalchemy/pool/base.py\", line 449, in connect\n    return _ConnectionFairy._checkout(self)\n" +
			"  File \"/usr/local/lib/python3.12/site-packages/sqlalchemy/pool/impl.py\", line 167, in _do_get\n    raise exc.TimeoutError(\n" +
			"sqlalchemy.exc.TimeoutError: " + pdbPoolTimeout})
		r.Status(500)
	}

	checkout := api.Endpoint("POST", "/api/checkout")
	checkout.Traffic(seed.Daily(1500), seed.RateFunc(load), seed.Over(window))
	checkout.Detail(seed.DetailPolicy{Recent: 10 * time.Hour, Rate: 0.03, Max: 14})
	checkout.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.8, 2.2)) // auth dependency, body validation
		wait, ok := waitFor(r)
		r.Gap(wait)
		cart := fmt.Sprintf("%08x-%04x-4%03x-%04x-%06x%06x", r.IntN(1<<30)|1<<28, r.IntN(1<<16), r.IntN(1<<12), 0x8000+r.IntN(1<<14), r.IntN(1<<24), r.IntN(1<<24))
		if !ok {
			r.Log(seed.SevWarn, "checkout failed for cart "+cart+": "+pdbPoolTimeout, seed.A("cart_id", cart))
			poolTimeout(r, "  File \"/app/app/checkout.py\", line 32, in create_checkout\n    return _checkout(body, x_user_id)\n"+
				"  File \"/app/app/checkout.py\", line 40, in _checkout\n    items = session.execute(select(CartItem).where(CartItem.cart_id == body.cart_id)).scalars().all()\n")
			return
		}
		lines := 1 + r.IntN(5)
		r.DB(pdbCartItems, r.Ms(2, 5), seed.Rows(lines))
		r.DB(pdbStockQuery(lines), r.Ms(40, 60), seed.Rows(lines))
		r.HTTP("POST", "https://api.payments.example/v1/charges", r.Ms(150, 260), 201)
		r.DB("INSERT INTO orders (id, cart_id, user_id, total_cents, status) VALUES (%(id)s::UUID, %(cart_id)s::UUID, %(user_id)s::INTEGER, %(total_cents)s::INTEGER, %(status)s::VARCHAR) RETURNING orders.created_at", r.Ms(4, 8))
		for range lines {
			r.DB("UPDATE inventory SET available=(inventory.available - %(available_1)s::INTEGER) WHERE inventory.sku = %(sku_1)s::VARCHAR", r.Ms(3, 6))
		}
		r.DB("DELETE FROM cart_items WHERE cart_items.cart_id = %(cart_id_1)s::UUID", r.Ms(2, 4))
		r.Gap(r.Ms(0.5, 1.5))
		r.Status(201)
	})

	cartEp := api.Endpoint("GET", "/api/cart")
	cartEp.Traffic(seed.Daily(1100), seed.RateFunc(load), seed.Over(window))
	cartEp.Detail(seed.DetailPolicy{Recent: 10 * time.Hour, Rate: 0.02, Max: 8})
	cartEp.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.6, 1.6))
		wait, ok := waitFor(r)
		r.Gap(wait)
		if !ok {
			poolTimeout(r, "  File \"/app/app/cart.py\", line 18, in get_cart\n    rows = session.execute(select(CartItem).where(CartItem.cart_id == cart_id)).scalars().all()\n")
			return
		}
		r.DB(pdbCartItems, r.Ms(2, 5), seed.Rows(1+r.IntN(5)))
		r.Gap(r.Ms(0.4, 1.0))
	})

	// Shipping options come from config; no connection needed.
	shipping := api.Endpoint("GET", "/api/shipping/options")
	shipping.Traffic(seed.Daily(500), seed.RateFunc(load), seed.Over(window))
	shipping.Detail(seed.DetailPolicy{Recent: 10 * time.Hour, Rate: 0.01, Max: 3})
	shipping.Handler(func(r *seed.Req) { r.Gap(r.Ms(0.6, 1.8)) })

	// opentelemetry-instrumentation-sqlalchemy's pool usage metric; the pool
	// is named after the engine URL.
	used := func(t time.Time) float64 {
		if over(t) > 0 {
			return 15
		}
		busy := 1 + 13*math.Pow((load(t)-1)/1.7, 1.4)*(0.9+0.2*math.Sin(float64(t.Unix()/600)))
		return math.Min(14, math.Round(busy))
	}
	withState := func(state string) map[string]any {
		return map[string]any{"state": state, "pool.name": "postgresql+psycopg://checkout-db.internal:5432/checkout"}
	}
	api.Gauge("db.client.connections.usage", "connections", withState("used"), used)
	api.Gauge("db.client.connections.usage", "connections", withState("idle"), func(t time.Time) float64 { return math.Max(0, 10-used(t)) })

	api.Host().CPU(seed.Plus(seed.Flat(0.27, 0.34), func(t time.Time) float64 { return 0.05 * bump(t) }))
	api.Host().Memory(seed.Flat(0.49, 0.55))

	day := s.Ago(24 * time.Hour)
	// A slow one a customer would notice (10+ s), not a request that waited
	// out the whole pool_timeout.
	s.Expose("slowRequest", pdbSlowest(checkout, day, func(r *seed.Request) bool {
		return r.Status == 201 && r.Duration() < 15*time.Second
	}))
	s.Expose("p50Ms24h", pdbMs(checkout, 0.5, day, s.Now))
	s.Expose("p95Ms24h", pdbMs(checkout, 0.95, day, s.Now))
	s.Expose("p99Ms24h", pdbMs(checkout, 0.99, day, s.Now))
	s.Expose("cartP99Ms24h", pdbMs(cartEp, 0.99, day, s.Now))
	s.Expose("timeouts", func() any {
		n := 0
		for _, e := range []*seed.Endpoint{checkout, cartEp} {
			for _, r := range e.Requests() {
				if r.Status == 500 {
					n++
				}
			}
		}
		return n
	})
	return nil
}
