package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/infra-disk-io", papSeedDiskIO) }

// papSeedDiskIO: ~9h40m ago a Kubernetes Job started rebuilding the
// search_documents table on the bookings database. Ever since, the
// database host's disk runs at its limit (reads 15 -> ~110MB/s, writes 8 ->
// ~55MB/s, ~35 operations queued, iowait ~2% -> ~35%) while its CPU stays
// low, and every query of bookings-api, indexed primary-key lookups
// included, takes 5-8x longer. The API's own CPU is flat. Distractor: a
// missing index on the slowest query (the availability search); but every
// query slowed by the same factor at the same minute, with no deploy of the
// API and no growth in its tables.
func papSeedDiskIO(s *seed.Scenario) error {
	p := s.Project("bookings", seed.OTel)
	api := p.Service("bookings-api", seed.Node("express", "pg")).Versions("6.4.1").
		Instances("bookings-api-6d8f9c7b4-2kqxp", "bookings-api-6d8f9c7b4-h7w5n", "bookings-api-6d8f9c7b4-tz9mc")
	db := p.Service("bookings-db", seed.Go("nethttp", "pgx")).Instances("pg-bookings-1")
	window := 24 * time.Hour
	onset := s.Ago(9*time.Hour + 40*time.Minute)

	slow := func(r *seed.Req, d time.Duration, write bool) time.Duration {
		if r.Start().Before(onset) {
			return d
		}
		f := 4.5 + 3*r.Float()
		if write {
			f += 2 // commits wait on the WAL fsync too
		}
		return time.Duration(float64(d) * f)
	}
	detail := seed.DetailPolicy{Recent: 20 * time.Hour, Rate: 0.03, Max: 30}

	avail := api.Endpoint("GET", "/api/availability")
	avail.Traffic(papJitter("avail", 0.06), seed.Daily(2200), seed.Diurnal(), seed.Over(window))
	avail.Detail(detail)
	avail.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.2))
		r.DB("SELECT listing_id, day, units_free, price_cents FROM availability WHERE listing_id = ANY($1) AND day BETWEEN $2 AND $3 ORDER BY listing_id, day",
			slow(r, r.Ms(7, 16), false), seed.Rows(120+r.IntN(400)))
		r.Gap(r.Ms(1.5, 4))
	})

	booking := api.Endpoint("GET", "/api/bookings/:id")
	booking.Traffic(papJitter("booking", 0.06), seed.Daily(1300), seed.Diurnal(), seed.Over(window))
	booking.Detail(detail)
	booking.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.2))
		r.DB("SELECT id, listing_id, guest_id, check_in, check_out, status, total_cents FROM bookings WHERE id = $1", slow(r, r.Ms(1, 2.8), false), seed.Rows(1))
		r.DB("SELECT id, title, city, host_id FROM listings WHERE id = $1", slow(r, r.Ms(0.9, 2.5), false), seed.Rows(1))
		r.Gap(r.Ms(0.3, 0.9))
	})

	create := api.Endpoint("POST", "/api/bookings")
	create.Traffic(papJitter("create", 0.06), seed.Daily(500), seed.Diurnal(), seed.Over(window))
	create.Detail(detail)
	create.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.5))
		r.DB("BEGIN", r.Ms(0.2, 0.5))
		r.DB("SELECT units_free FROM availability WHERE listing_id = $1 AND day >= $2 AND day < $3 FOR UPDATE", slow(r, r.Ms(1.5, 4), false))
		r.DB("UPDATE availability SET units_free = units_free - 1 WHERE listing_id = $1 AND day >= $2 AND day < $3", slow(r, r.Ms(1.5, 4), true))
		r.DB("INSERT INTO bookings (id, listing_id, guest_id, check_in, check_out, status, total_cents, created_at) VALUES ($1, $2, $3, $4, $5, 'confirmed', $6, now())", slow(r, r.Ms(1.2, 3), true))
		r.DB("COMMIT", slow(r, r.Ms(1.5, 4), true))
		r.Status(201)
	})

	reviews := api.Endpoint("GET", "/api/listings/:id/reviews")
	reviews.Traffic(papJitter("reviews", 0.06), seed.Daily(900), seed.Diurnal(), seed.Over(window))
	reviews.Detail(detail)
	reviews.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.2))
		r.DB("SELECT id, author_name, rating, body, created_at FROM reviews WHERE listing_id = $1 ORDER BY created_at DESC LIMIT 20", slow(r, r.Ms(2.5, 7), false), seed.Rows(20))
		r.Gap(r.Ms(0.8, 2))
	})

	for i, h := range api.Hosts {
		shift := time.Duration(i) * 137 * time.Second // pods do not share one noise curve
		api.HostNamed(h).CPU(func(t time.Time) float64 { return seed.Flat(0.17, 0.25)(t.Add(shift)) })
		api.HostNamed(h).Memory(func(t time.Time) float64 { return seed.Flat(0.39, 0.44)(t.Add(shift)) })
	}

	pg := db.Host()
	pg.CPU(seed.Step(onset, seed.Flat(0.1, 0.15), seed.Flat(0.14, 0.2)),
		seed.CPUExtra{Wait: seed.Step(onset, seed.Flat(0.012, 0.028), seed.Flat(0.3, 0.4))})
	pg.Disk("nvme1n1",
		seed.Step(onset, seed.Flat(1.1e7, 1.9e7), seed.Flat(1.04e8, 1.16e8)),
		seed.Step(onset, seed.Flat(5e6, 1.1e7), seed.Flat(4.8e7, 6.2e7)),
		seed.Step(onset, seed.Flat(0.4, 1.6), seed.Flat(28, 44)))
	pg.Memory(seed.Flat(0.61, 0.64))
	pg.Filesystem("/var/lib/postgresql", seed.Plus(seed.Flat(0.57, 0.58), func(t time.Time) float64 {
		if t.Before(onset) {
			return 0
		}
		return 0.006 * t.Sub(onset).Hours() // the rebuilt table grows on the same volume
	}))

	rfc := func(t time.Time) string { return t.UTC().Format(time.RFC3339) }
	s.Expose("onsetAt", rfc(onset))
	s.Expose("beforeFrom", rfc(onset.Add(-6*time.Hour)))
	s.Expose("afterTo", rfc(onset.Add(6*time.Hour)))
	s.Expose("slowAvailability", avail.Pick(func(rq *seed.Request) bool {
		return rq.Start.After(onset.Add(time.Hour)) && rq.Duration() > 60*time.Millisecond
	}))
	s.Expose("slowBooking", booking.Pick(func(rq *seed.Request) bool {
		return rq.Start.After(onset.Add(time.Hour)) && rq.Duration() > 15*time.Millisecond
	}))
	s.Expose("earlierBooking", booking.Pick(func(rq *seed.Request) bool {
		return rq.Start.Before(onset.Add(-time.Hour)) && rq.Start.After(onset.Add(-8*time.Hour))
	}))
	return nil
}
