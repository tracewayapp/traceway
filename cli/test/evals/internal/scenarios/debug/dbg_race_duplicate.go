package debug

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/race-duplicate", dbgSeedRaceDuplicate) }

type dbgSubmit struct {
	req      *seed.Request
	cartID   string
	insertAt time.Duration // offset of the order INSERT in the first request
	endAt    time.Duration
	client   string
	ua       string
}

// dbgSeedRaceDuplicate: about one checkout in eighty is submitted twice
// (double click, a phone resending a slow request). The first request
// creates the order, charges the card and queues the confirmation; the
// second arrives while the charge is running, hits the orders_cart_id_key
// unique constraint and answers 500, which is the page the shopper sees.
// Distractor: a short connection-timeout burst on GET /api/orders/:orderId
// three days ago.
func dbgSeedRaceDuplicate(s *seed.Scenario) error {
	p := s.Project("storefront", seed.OTel)
	api := p.Service("checkout-api", seed.Node("express", "pg")).Versions("4.2.0")
	window := 7 * 24 * time.Hour
	dupStack := "error: duplicate key value violates unique constraint \"orders_cart_id_key\"\n" +
		"    at /app/node_modules/pg-pool/index.js:45:11\n" +
		"    at process.processTicksAndRejections (node:internal/process/task_queues:105:5)\n" +
		"    at async PgOrdersRepo.insertOrder (/app/src/orders/pgRepo.js:21:22)\n" +
		"    at async placeOrder (/app/src/orders/service.js:20:17)\n" +
		"    at async /app/src/orders/routes.js:11:21"
	poolStack := "Error: Connection terminated due to connection timeout\n" +
		"    at /app/node_modules/pg-pool/index.js:45:11\n" +
		"    at process.processTicksAndRejections (node:internal/process/task_queues:105:5)\n" +
		"    at async PgOrdersRepo.findOrder (/app/src/orders/pgRepo.js:29:22)\n" +
		"    at async /app/src/orders/routes.js:25:42"
	blipFrom := s.Ago(3*24*time.Hour + 4*time.Hour).Truncate(time.Minute)
	blipTo := blipFrom.Add(6 * time.Minute)

	var submits []*dbgSubmit
	checkout := func(r *seed.Req, cartID string) (orderID string, insertAt time.Duration) {
		r.RootAttr("cart.id", cartID)
		r.Gap(r.Ms(0.5, 1.4))
		r.DB("SELECT id, user_id, currency FROM carts WHERE id = $1", r.Ms(1, 3))
		r.DB("SELECT sku, qty, unit_price_cents FROM cart_items WHERE cart_id = $1", r.Ms(1, 3), seed.Rows(1+r.IntN(5)))
		insertAt = r.At().Sub(r.Start())
		r.DB("INSERT INTO orders (cart_id, user_id, total_cents, currency, status, created_at) VALUES ($1, $2, $3, $4, 'pending', now()) RETURNING id, cart_id, user_id, total_cents, currency, status", r.Ms(2, 4))
		return fmt.Sprintf("ord_%d", 5_200_000+r.IntN(800_000)), insertAt
	}
	finish := func(r *seed.Req, cartID, orderID string, log bool) {
		r.HTTP("POST", "https://api.payments.example/v1/charges", r.LogNormal(520*time.Millisecond, 1400*time.Millisecond), 200)
		r.DB("UPDATE orders SET status = 'paid', charge_id = $2 WHERE id = $1", r.Ms(1, 3))
		r.DB("INSERT INTO outbox (kind, ref, created_at) VALUES ('order_confirmation', $1, now())", r.Ms(1, 3))
		if log {
			r.Log(seed.SevInfo, fmt.Sprintf("order %s placed for cart %s", orderID, cartID))
		}
		r.Gap(r.Ms(0.3, 0.8))
		r.Status(201)
	}
	cartOf := func(r *seed.Req) string { return fmt.Sprintf("c_%06x", r.IntN(1<<24)) }

	place := api.Endpoint("POST", "/api/orders")
	place.Traffic(seed.Daily(1100), seed.Diurnal(), seed.Over(window))
	place.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 15})
	place.Handler(func(r *seed.Req) {
		cartID := cartOf(r)
		orderID, insertAt := checkout(r, cartID)
		double := r.Chance(0.013)
		finish(r, cartID, orderID, double || r.Chance(0.01))
		if double {
			rq := r.Request()
			submits = append(submits, &dbgSubmit{req: rq, cartID: cartID, insertAt: insertAt, endAt: r.At().Sub(r.Start()),
				client: rq.Root.Attrs["client.address"].(string), ua: rq.Root.Attrs["user_agent.original"].(string)})
		}
	})

	// The second submits: same route, each pinned into the charge of its
	// first submit, from the same browser.
	again := api.Endpoint("POST", "/api/orders")
	again.Traffic(seed.Daily(22), seed.Over(window))
	next := 0
	again.Handler(func(r *seed.Req) {
		place.Requests()
		if next >= len(submits) {
			cartID := cartOf(r)
			orderID, _ := checkout(r, cartID)
			finish(r, cartID, orderID, false)
			return
		}
		first := submits[next]
		next++
		rq := r.Request()
		lo, hi := first.insertAt+120*time.Millisecond, first.endAt-60*time.Millisecond
		if hi <= lo {
			hi = lo + 10*time.Millisecond
		}
		rq.Start = first.req.Start.Add(lo + time.Duration(r.Float()*float64(hi-lo)))
		rq.Root.Attrs["client.address"] = first.client
		rq.Root.Attrs["user_agent.original"] = first.ua
		rq.Root.Attrs["cart.id"] = first.cartID
		r.Gap(r.Ms(0.5, 1.4))
		r.DB("SELECT id, user_id, currency FROM carts WHERE id = $1", r.Ms(1, 3))
		r.DB("SELECT sku, qty, unit_price_cents FROM cart_items WHERE cart_id = $1", r.Ms(1, 3), seed.Rows(1+r.IntN(5)))
		ins := r.DB("INSERT INTO orders (cart_id, user_id, total_cents, currency, status, created_at) VALUES ($1, $2, $3, $4, 'pending', now()) RETURNING id, cart_id, user_id, total_cents, currency, status", r.Ms(1, 3))
		ins.Status, ins.StatusMsg = seed.StatusError, `duplicate key value violates unique constraint "orders_cart_id_key"`
		r.Error(seed.Exc{Type: "23505", Message: `duplicate key value violates unique constraint "orders_cart_id_key"`, Stack: dupStack})
		r.Log(seed.SevError, `request failed: duplicate key value violates unique constraint "orders_cart_id_key"`)
		r.Status(500)
		r.Gap(r.Ms(0.2, 0.6))
	})

	get := api.Endpoint("GET", "/api/orders/:orderId")
	get.Traffic(seed.Daily(600), seed.Diurnal(), seed.Burst(blipFrom, blipTo, 10), seed.Over(window))
	get.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.9))
		at := r.Start()
		if !at.Before(blipFrom) && at.Before(blipTo) && r.Chance(0.6) {
			r.Gap(r.Ms(5000, 5050))
			r.Error(seed.Exc{Type: "Error", Message: "Connection terminated due to connection timeout", Stack: poolStack})
			r.Log(seed.SevError, "request failed: Connection terminated due to connection timeout")
			r.Status(500)
			return
		}
		r.DB("SELECT id, cart_id, user_id, total_cents, currency, status FROM orders WHERE id = $1", r.Ms(1, 4))
		r.Gap(r.Ms(0.2, 0.6))
	})
	s.Background(api, seed.TypicalTraffic(3))
	api.Host().CPU(seed.Flat(0.18, 0.3))

	s.Expose("pairs", func() any { return len(submits) })
	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(8*24*time.Hour, "orders_cart_id_key")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		other, err := p.GroupMatching(8*24*time.Hour, "connection timeout")
		if err != nil {
			return err
		}
		s.Expose("otherHash", other)
		occs, err := dbgOccurrences(s, p, hash, 3)
		if err != nil {
			return err
		}
		if len(occs) > 0 {
			ref := dbgOccRef(occs[0])
			ref["cart"] = occs[0].Attributes["cart.id"]
			s.Expose("latest", ref)
		}
		return nil
	})
	return nil
}
