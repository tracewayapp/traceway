package perf

import (
	"fmt"
	"math"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/db-lock-contention", pdbSeedLockContention) }

// pdbSeedLockContention: every evening at 18:00 UTC a product drop sends
// reservations for the same few SKUs. A reservation updates its cart's
// inventory rows one by one, in cart order, inside one transaction, so
// concurrent reservations queue on the hot rows' locks: the UPDATE inventory
// spans themselves run for hundreds of ms to seconds, the transaction starts
// at once (no gap before the first query), and carts that lock two hot SKUs
// in opposite orders deadlock (40P01 after deadlock_timeout) or give up at
// lock_timeout (55P03). Distractor: connection-pool exhaustion. Release
// 1.32.0 a week ago raised the pgx pool from 20 to 40 connections and the
// evening spikes did not change; there is no wait before BEGIN.
func pdbSeedLockContention(s *seed.Scenario) error {
	// Nine days, so the 42 hours before the deploy hold a full evening drop
	// whatever the hour of the run.
	window := 9 * 24 * time.Hour
	deployAt := s.Ago(7*24*time.Hour + 6*time.Hour)
	p := s.Project("fulfillment", seed.OTel)
	api := p.Service("inventory-api", seed.Go("nethttp", "pgx")).
		Versions("1.31.4", seed.Deploy{At: deployAt, Version: "1.32.0"})

	// heat is 1 during the drop (18:00-18:25 UTC) and fades out by 18:45.
	heat := func(t time.Time) float64 {
		m := float64(t.Hour()*60+t.Minute()) + float64(t.Second())/60
		switch {
		case m >= 18*60 && m < 18*60+25:
			return 1
		case m >= 18*60+25 && m < 18*60+45:
			return (18*60 + 45 - m) / 20
		}
		return 0
	}
	load := func(t time.Time) float64 { return 1 + 6*heat(t) }

	hot := []string{"SKU-DROP-1187", "SKU-DROP-1188", "SKU-DROP-1190"}
	pickSKU := func(r *seed.Req, h float64) string {
		if h > 0 && r.Chance(0.85) {
			return hot[r.IntN(len(hot))]
		}
		return fmt.Sprintf("SKU-%05d", 10000+r.IntN(80000))
	}
	const updateSQL = "UPDATE inventory SET reserved = reserved + $1, updated_at = now() WHERE sku = $2 AND warehouse_id = $3 AND on_hand - reserved >= $1"
	pid := func(r *seed.Req) int { return 41000 + r.IntN(9000) }
	tuple := func(r *seed.Req) string {
		return fmt.Sprintf("while updating tuple (%d,%d) in relation \"inventory\"", 1800+r.IntN(60), 1+r.IntN(40))
	}

	reserve := api.Endpoint("POST", "/api/reservations")
	reserve.Traffic(seed.Daily(700), seed.RateFunc(load), seed.Over(window))
	reserve.Detail(seed.DetailPolicy{Recent: 10 * time.Hour, Rate: 0.03, Max: 10})
	reserve.Handler(func(r *seed.Req) {
		h := heat(r.Start())
		lines := 1 + r.IntN(4)
		cart := fmt.Sprintf("c_%08x", r.IntN(1<<31))
		r.Gap(r.Ms(0.3, 0.9)) // decode the cart
		r.DB("begin", r.Ms(0.2, 0.6))
		r.DB("SET LOCAL lock_timeout = '5s'", r.Ms(0.1, 0.3))
		hotLocked := 0
		for i := 0; i < lines; i++ {
			sku := pickSKU(r, h)
			isHot := len(sku) > 8 && sku[:8] == "SKU-DROP"
			d := r.Ms(0.6, 1.8)
			if isHot && h > 0 && r.Chance(0.4+0.5*h) {
				// Waiting for the row lock held by another reservation.
				d += time.Duration(h * float64(r.LogNormal(380*time.Millisecond, 2800*time.Millisecond)))
			}
			if isHot {
				hotLocked++
			}
			switch {
			case d >= 5*time.Second:
				sp := r.DB(updateSQL, 5*time.Second+r.Ms(0.5, 2))
				pdbPgError(r, sp, "ERROR: canceling statement due to lock timeout (SQLSTATE 55P03)")
				r.DB("rollback", r.Ms(0.2, 0.6))
				r.Log(seed.SevWarn, "reserve stock: ERROR: canceling statement due to lock timeout (SQLSTATE 55P03)",
					seed.A("cart_id", cart), seed.A("sku", sku), seed.A("pg_where", tuple(r)))
				r.Status(503)
				return
			case hotLocked >= 2 && h > 0 && r.Chance(0.07):
				// Another cart holds this row and waits for one we hold.
				sp := r.DB(updateSQL, time.Second+r.Ms(1, 6))
				pdbPgError(r, sp, "ERROR: deadlock detected (SQLSTATE 40P01)")
				r.DB("rollback", r.Ms(0.2, 0.6))
				a, b := pid(r), pid(r)
				tx1, tx2 := 9917000+r.IntN(900), 9917000+r.IntN(900)
				r.Log(seed.SevError, "reserve stock: ERROR: deadlock detected (SQLSTATE 40P01)",
					seed.A("cart_id", cart), seed.A("sku", sku),
					seed.A("pg_detail", fmt.Sprintf("Process %d waits for ShareLock on transaction %d; blocked by process %d.\nProcess %d waits for ShareLock on transaction %d; blocked by process %d.", a, tx1, b, b, tx2, a)),
					seed.A("pg_where", tuple(r)))
				r.Status(500)
				return
			}
			r.DB(updateSQL, d, seed.Rows(1))
		}
		r.DB("INSERT INTO reservations (cart_id, sku, warehouse_id, qty, expires_at) SELECT $1, unnest($2::text[]), $3, unnest($4::int[]), now() + interval '15 minutes'", r.Ms(0.8, 2.2))
		r.DB("commit", r.Ms(0.8, 2.6))
		r.Gap(r.Ms(0.2, 0.5))
		r.Status(201)
	})

	release := api.Endpoint("POST", "/api/reservations/{id}/release")
	release.Traffic(seed.Daily(160), seed.RateFunc(load), seed.Over(window))
	release.Detail(seed.DetailPolicy{Recent: 10 * time.Hour, Rate: 0.02, Max: 3})
	release.Handler(func(r *seed.Req) {
		h := heat(r.Start())
		r.Gap(r.Ms(0.3, 0.8))
		r.DB("begin", r.Ms(0.2, 0.6))
		r.DB("DELETE FROM reservations WHERE id = $1 RETURNING sku, warehouse_id, qty", r.Ms(0.6, 1.6), seed.Rows(1))
		d := r.Ms(0.6, 1.8)
		if h > 0 && r.Chance(0.5*h) {
			d += time.Duration(h * float64(r.LogNormal(250*time.Millisecond, 2*time.Second)))
		}
		r.DB("UPDATE inventory SET reserved = reserved - $1, updated_at = now() WHERE sku = $2 AND warehouse_id = $3", time.Duration(math.Min(float64(d), float64(4900*time.Millisecond))), seed.Rows(1))
		r.DB("commit", r.Ms(0.8, 2.4))
		r.Gap(r.Ms(0.2, 0.5))
	})

	pdbHealthy(api, window,
		pdbRoute{method: "GET", route: "/api/inventory/{sku}", perDay: 220, queries: []string{
			"SELECT sku, warehouse_id, on_hand, reserved, updated_at FROM inventory WHERE sku = $1",
		}, dbMedianMs: 0.9},
		pdbRoute{method: "GET", route: "/api/warehouses", perDay: 30, queries: []string{
			"SELECT id, code, name, timezone FROM warehouses ORDER BY code",
		}, dbMedianMs: 0.8},
	)
	api.Host().CPU(seed.Plus(seed.Flat(0.16, 0.24), func(t time.Time) float64 { return 0.04 * heat(t) }))
	api.Host().Memory(seed.Flat(0.38, 0.43))

	day := s.Ago(24 * time.Hour)
	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	s.Expose("slowRequest", pdbSlowest(reserve, day, func(r *seed.Request) bool { return r.Status == 201 }))
	s.Expose("p50Ms24h", pdbMs(reserve, 0.5, day, s.Now))
	s.Expose("p99Ms24h", pdbMs(reserve, 0.99, day, s.Now))
	s.Expose("failures", func() any {
		n := 0
		for _, r := range reserve.Requests() {
			if r.Status >= 500 {
				n++
			}
		}
		return n
	})
	return nil
}

// pdbPgError records the error otelpgx attaches to a failed query span.
func pdbPgError(r *seed.Req, sp *seed.Span, msg string) {
	sp.Status = seed.StatusError
	sp.StatusMsg = msg
	sp.Events = append(sp.Events, seed.Event{Name: "exception", At: sp.Dur, Attrs: map[string]any{
		"exception.type": "*pgconn.PgError", "exception.message": msg,
	}})
	r.KeepDetail()
}
