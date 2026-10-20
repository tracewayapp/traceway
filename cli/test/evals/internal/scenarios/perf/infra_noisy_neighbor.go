package perf

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/infra-noisy-neighbor", papSeedNoisyNeighbor) }

// papSeedNoisyNeighbor: tickets-api runs on three VMs behind a round-robin
// load balancer. ~14h ago the hypervisor under tickets-node-c got busy:
// that VM now loses 25-35% of its CPU time to steal and every request it
// serves takes ~3x longer, while node-a and node-b are unchanged. Endpoint
// rows mix all three hosts, so the p95 of every endpoint rose, but only
// because a third of the requests are slow. Per-host latency is visible in
// otelhttp's http.server.request.duration grouped by host.name, and in the
// access logs. Distractor: fleet-wide CPU saturation; but node-a and node-b
// are ~70% idle, node-c's own user+system time did not grow, and traffic is
// flat.
func papSeedNoisyNeighbor(s *seed.Scenario) error {
	p := s.Project("ticketing", seed.OTel)
	hosts := []string{"tickets-node-a", "tickets-node-b", "tickets-node-c"}
	api := p.Service("tickets-api", seed.Go("nethttp", "pgx")).Versions("3.2.7").Instances(hosts...)
	window := 2 * 24 * time.Hour
	onset := s.Ago(14*time.Hour + 20*time.Minute)
	bad := hosts[2]

	stretched := func(r *seed.Req) bool { return r.Host() == bad && !r.Start().Before(onset) }
	cpu := func(r *seed.Req, d time.Duration) time.Duration {
		if stretched(r) {
			return time.Duration(float64(d) * (2.6 + 0.9*r.Float()))
		}
		return d
	}
	db := func(r *seed.Req, d time.Duration) time.Duration {
		if stretched(r) {
			return time.Duration(float64(d) * (1.5 + 0.5*r.Float()))
		}
		return d
	}
	detail := seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.06, Max: 30}

	events := api.Endpoint("GET", "/api/events")
	events.Traffic(papJitter("events", 0.06), seed.Daily(1300), seed.Diurnal(), seed.Over(window))
	events.Detail(detail)
	events.Handler(func(r *seed.Req) {
		r.Gap(cpu(r, r.Ms(0.5, 1.2)))
		r.DB("SELECT id, title, venue_id, starts_at, min_price_cents FROM events WHERE city_id = $1 AND starts_at > now() ORDER BY starts_at LIMIT 30", db(r, r.Ms(4, 10)), seed.Rows(30))
		r.Gap(cpu(r, r.Ms(6, 14))) // render the listing JSON
	})
	event := api.Endpoint("GET", "/api/events/:id")
	event.Traffic(papJitter("event", 0.06), seed.Daily(1100), seed.Diurnal(), seed.Over(window))
	event.Detail(detail)
	event.Handler(func(r *seed.Req) {
		r.Gap(cpu(r, r.Ms(0.5, 1.2)))
		r.DB("SELECT id, title, description, venue_id, starts_at FROM events WHERE id = $1", db(r, r.Ms(1.2, 3)))
		r.DB("SELECT section, row_label, seat_count, price_cents, available FROM seat_blocks WHERE event_id = $1", db(r, r.Ms(3, 8)), seed.Rows(60))
		r.Gap(cpu(r, r.Ms(8, 18))) // seat map
	})
	reserve := api.Endpoint("POST", "/api/orders")
	reserve.Traffic(papJitter("reserve", 0.06), seed.Daily(320), seed.Diurnal(), seed.Over(window))
	reserve.Detail(detail)
	reserve.Handler(func(r *seed.Req) {
		r.Gap(cpu(r, r.Ms(0.8, 2)))
		r.DB("UPDATE seat_blocks SET available = available - $2 WHERE id = $1 AND available >= $2", db(r, r.Ms(2, 5)))
		r.DB("INSERT INTO orders (id, event_id, buyer_id, seats, total_cents, status, created_at) VALUES ($1, $2, $3, $4, $5, 'reserved', now())", db(r, r.Ms(2, 5)))
		r.Gap(cpu(r, r.Ms(3, 8))) // sign the reservation token
		r.Status(201)
	})
	order := api.Endpoint("GET", "/api/orders/:id")
	order.Traffic(papJitter("order", 0.06), seed.Daily(500), seed.Diurnal(), seed.Over(window))
	order.Detail(detail)
	order.Handler(func(r *seed.Req) {
		r.Gap(cpu(r, r.Ms(0.5, 1.2)))
		r.DB("SELECT id, event_id, buyer_id, seats, total_cents, status FROM orders WHERE id = $1", db(r, r.Ms(1.2, 3)))
		r.Gap(cpu(r, r.Ms(1, 3)))
	})

	// otelhttp's http.server.request.duration, per host and method: the
	// mean and count of the requests since the previous point, from the
	// generated requests. Points come every minute in the last day and every
	// ten before that (seed.MetricStep), so each point covers one step; a
	// per-minute share of a longer window would round to no count at night
	// and drop the point.
	type agg struct {
		sum time.Duration
		n   int
	}
	var perMin map[string]map[time.Time]*agg
	build := func() {
		perMin = map[string]map[time.Time]*agg{}
		for _, e := range []*seed.Endpoint{events, event, reserve, order} {
			for _, rq := range e.Requests() {
				k := rq.Host + "|" + e.Method
				if perMin[k] == nil {
					perMin[k] = map[time.Time]*agg{}
				}
				m := rq.Start.Truncate(time.Minute)
				a := perMin[k][m]
				if a == nil {
					a = &agg{}
					perMin[k][m] = a
				}
				a.sum += rq.Duration()
				a.n++
			}
		}
	}
	recentCut := s.Now.Add(-time.Minute - 24*time.Hour)
	step := func(t time.Time) time.Duration {
		if t.Before(recentCut) {
			return seed.MetricStep.Old
		}
		return seed.MetricStep.Recent
	}
	for _, h := range hosts {
		for _, method := range []string{"GET", "POST"} {
			k := h + "|" + method
			attrs := map[string]any{"http.request.method": method, "http.response.status_code": 200, "url.scheme": "https", "network.protocol.version": "1.1"}
			if method == "POST" {
				attrs["http.response.status_code"] = 201
			}
			windowAt := func(t time.Time) agg {
				if perMin == nil {
					build()
				}
				var out agg
				for m := t.Add(-step(t)); m.Before(t); m = m.Add(time.Minute) {
					if a := perMin[k][m]; a != nil {
						out.sum += a.sum
						out.n += a.n
					}
				}
				return out
			}
			mean := func(t time.Time) float64 {
				if a := windowAt(t); a.n > 0 {
					return (a.sum / time.Duration(a.n)).Seconds()
				}
				return 0
			}
			perMinute := func(t time.Time) float64 {
				return float64(windowAt(t).n) / step(t).Minutes()
			}
			api.HistogramOn(h, "http.server.request.duration", "s", attrs, mean, perMinute)
		}
	}

	for _, h := range hosts {
		host := api.HostNamed(h)
		if h == bad {
			host.CPU(seed.Flat(0.27, 0.35), seed.CPUExtra{Steal: seed.Step(onset, seed.Flat(0, 0.004), seed.Flat(0.24, 0.34))})
		} else {
			host.CPU(seed.Flat(0.26, 0.34))
		}
		host.Memory(seed.Flat(0.47, 0.53))
	}

	rfc := func(t time.Time) string { return t.UTC().Format(time.RFC3339) }
	s.Expose("onsetAt", rfc(onset))
	s.Expose("slowRequest", event.Pick(func(rq *seed.Request) bool {
		return rq.Host == bad && rq.Start.After(s.Ago(6*time.Hour)) && rq.Duration() > 45*time.Millisecond
	}))
	s.Expose("fastRequest", event.Pick(func(rq *seed.Request) bool {
		return rq.Host == hosts[0] && rq.Start.After(s.Ago(6*time.Hour)) && rq.Duration() < 25*time.Millisecond
	}))
	s.Expose("eventP95Before", func() any {
		return event.PercentileBetween(0.95, onset.Add(-8*time.Hour), onset).Seconds() * 1000
	})
	s.Expose("eventP95After", func() any {
		return event.PercentileBetween(0.95, onset, s.Now).Seconds() * 1000
	})
	return nil
}
