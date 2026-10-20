package perf

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("perf/net-slow-third-party", pncSeedSlowThirdParty) }

type pncWindow struct{ from, to time.Time }

func (w pncWindow) has(t time.Time) bool { return !t.Before(w.from) && t.Before(w.to) }

func pncInAny(ws []pncWindow, t time.Time) bool {
	for _, w := range ws {
		if w.has(t) {
			return true
		}
	}
	return false
}

// pncSeedSlowThirdParty: checkout charges the card through an external
// payment provider. Three times in three days the provider slowed down for
// 25-40 minutes: the charge CLIENT span grew from ~230ms to seconds (some
// calls hit the 10s client timeout), and checkout and refunds spiked
// together while routes that do not call the provider stayed flat.
// Distractor: goroutines pile up during the spikes, which reads like pool
// exhaustion, but the DB spans start immediately and stay fast.
func pncSeedSlowThirdParty(s *seed.Scenario) error {
	window := 3 * 24 * time.Hour
	incidents := []pncWindow{
		{s.Ago(61*time.Hour + 12*time.Minute), s.Ago(61*time.Hour + 12*time.Minute - 35*time.Minute)},
		{s.Ago(30*time.Hour + 47*time.Minute), s.Ago(30*time.Hour + 47*time.Minute - 25*time.Minute)},
		{s.Ago(5*time.Hour + 21*time.Minute), s.Ago(5*time.Hour + 21*time.Minute - 40*time.Minute)},
	}
	p := s.Project("shop", seed.OTel)
	api := p.Service("checkout-api", seed.Go("nethttp", "pgx")).Versions("5.2.0")

	providerCall := func(r *seed.Req, path string, timeout bool) (time.Duration, bool) {
		d := r.LogNormal(230*time.Millisecond, 560*time.Millisecond)
		if pncInAny(incidents, r.At()) {
			d = r.LogNormal(4*time.Second, 16*time.Second)
		}
		if timeout {
			d = 12 * time.Second
		}
		url := "https://api.payments.example" + path
		if d >= 10*time.Second {
			sp := r.HTTP("POST", url, 10*time.Second, 0)
			delete(sp.Attrs, "http.response.status_code")
			sp.Attrs["error.type"] = "*url.Error"
			sp.Status = seed.StatusError
			sp.StatusMsg = fmt.Sprintf("Post %q: context deadline exceeded (Client.Timeout exceeded while awaiting headers)", url)
			return d, false
		}
		r.HTTP("POST", url, d, 200, seed.A("http.request.body.size", 380+r.IntN(200)))
		return d, true
	}

	checkout := api.Endpoint("POST", "/api/checkout")
	checkout.Traffic(seed.Daily(1500), seed.Diurnal(), seed.Over(window))
	checkout.Detail(seed.DetailPolicy{Recent: 8 * time.Hour, Rate: 0.03, Max: 20})
	forcedTimeout := false
	checkout.Handler(func(r *seed.Req) {
		// At least one charge in the latest window runs into the client timeout.
		timeout := !forcedTimeout && incidents[2].has(r.Start()) && !r.Start().Before(incidents[2].from.Add(10*time.Minute))
		forcedTimeout = forcedTimeout || timeout
		r.Gap(r.Ms(0.4, 1.0))
		r.DB("SELECT id, status, total_cents, currency, payment_method FROM orders WHERE id = $1 AND customer_id = $2", r.LogNormal(2500*time.Microsecond, 7*time.Millisecond))
		r.DB("SELECT sku, qty, unit_price_cents FROM order_lines WHERE order_id = $1", r.LogNormal(2*time.Millisecond, 5*time.Millisecond), seed.Rows(1+r.IntN(6)))
		if _, ok := providerCall(r, "/v1/charges", timeout); !ok {
			r.Error(seed.Exc{Type: "*url.Error", Message: `Post "https://api.payments.example/v1/charges": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`})
			r.Log(seed.SevError, "charge failed", seed.A("error", `Post "https://api.payments.example/v1/charges": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`))
			r.Status(502)
			r.Gap(r.Ms(0.2, 0.6))
			return
		}
		r.DB("UPDATE orders SET status = 'paid', paid_at = now(), charge_id = $2 WHERE id = $1", r.LogNormal(2500*time.Microsecond, 6*time.Millisecond))
		r.DB("INSERT INTO payment_events (order_id, kind, payload) VALUES ($1, $2, $3)", r.LogNormal(2*time.Millisecond, 5*time.Millisecond))
		r.Gap(r.Ms(0.3, 1))
	})

	refunds := api.Endpoint("POST", "/api/refunds")
	refunds.Traffic(seed.Daily(320), seed.Diurnal(), seed.Over(window))
	refunds.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.0))
		r.DB("SELECT id, status, charge_id, total_cents FROM orders WHERE id = $1", r.LogNormal(2500*time.Microsecond, 7*time.Millisecond))
		if _, ok := providerCall(r, "/v1/refunds", false); !ok {
			r.Status(502)
			return
		}
		r.DB("INSERT INTO payment_events (order_id, kind, payload) VALUES ($1, $2, $3)", r.LogNormal(2*time.Millisecond, 5*time.Millisecond))
	})

	order := api.Endpoint("GET", "/api/orders/{id}")
	order.Traffic(seed.Daily(900), seed.Diurnal(), seed.Over(window))
	order.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.8))
		r.DB("SELECT id, status, total_cents, currency, paid_at FROM orders WHERE id = $1", r.LogNormal(2500*time.Microsecond, 7*time.Millisecond))
		r.DB("SELECT sku, qty, unit_price_cents FROM order_lines WHERE order_id = $1", r.LogNormal(2*time.Millisecond, 5*time.Millisecond))
		r.Gap(r.Ms(0.3, 1))
	})

	s.Background(api, seed.TrafficOpts{Endpoints: 5, PerDay: 150})
	api.Host().CPU(seed.Flat(0.18, 0.27))
	api.Host().Memory(seed.Flat(0.46, 0.53))
	goroutines := func(t time.Time) float64 {
		if pncInAny(incidents, t) {
			return 380
		}
		return 54
	}
	api.Gauge("go.goroutine.count", "{goroutine}", nil, seed.Plus(goroutines, seed.Flat(0, 30)))

	s.Expose("incidentRequest", checkout.Pick(func(r *seed.Request) bool {
		return incidents[2].has(r.Start) && r.Status == 200 && r.Duration() > 3*time.Second && r.Duration() < 8*time.Second
	}))
	s.Expose("incidentFrom", incidents[2].from.Truncate(time.Hour).Format(time.RFC3339))
	s.Expose("incidentTo", incidents[2].to.Truncate(time.Hour).Add(time.Hour).Format(time.RFC3339))
	for i, w := range incidents {
		s.Expose(fmt.Sprintf("incident%d", i+1), w.from.Format(time.RFC3339))
	}
	s.Expose("p50Ms3d", func() any { return checkout.Percentile(0.5, window).Seconds() * 1000 })
	s.Expose("p99Ms3d", func() any { return checkout.Percentile(0.99, window).Seconds() * 1000 })
	return nil
}
