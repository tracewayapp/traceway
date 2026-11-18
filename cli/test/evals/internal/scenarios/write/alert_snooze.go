package write

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("write/alert-snooze", seedAlertSnooze) }

// checkoutTraffic is a payments service whose checkout calls a card
// processor. failingSince marks when the processor started returning 502s
// for a share of charges.
func checkoutTraffic(s *seed.Scenario, p *seed.Project, failingSince time.Time, failShare float64) (*seed.Service, *seed.Endpoint) {
	api := p.Service("payments-api", seed.Go("chi", "pgx")).Versions("3.4.1")
	checkout := api.Endpoint("POST", "/api/checkout")
	checkout.Traffic(seed.Daily(1800), seed.Diurnal(), seed.Burst(s.Ago(90*time.Minute), s.Now, 5), seed.Over(24*time.Hour))
	checkout.Detail(seed.DetailPolicy{Recent: time.Hour, Rate: 0.05, Max: 20})
	checkout.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.5))
		r.DB("SELECT id, user_id, total_cents, currency FROM carts WHERE id = $1 FOR UPDATE", r.Ms(2, 6))
		failing := !r.Start().Before(failingSince) && r.Chance(failShare)
		if failing {
			r.HTTP("POST", "https://api.cardprocessor.example/v2/charges", r.Ms(900, 2400), 502)
			r.Error(seed.Exc{Type: "*payments.GatewayError", Message: "charge declined by gateway: upstream returned 502 Bad Gateway",
				Stack: "goroutine 211 [running]:\nexample.com/payments-api/internal/gateway.(*Client).Charge(0xc0003a2000, {0x1c8e2a0, 0xc00041e0c0}, 0xc0004a4180)\n\t/app/internal/gateway/client.go:118 +0x3c5\nexample.com/payments-api/internal/checkout.(*Service).Pay(0xc000182a50, {0x1c8e2a0, 0xc00041e0c0}, 0xc0004a4180)\n\t/app/internal/checkout/pay.go:64 +0x1d2"})
			r.Status(502)
			r.Gap(r.Ms(0.3, 0.8))
			return
		}
		r.HTTP("POST", "https://api.cardprocessor.example/v2/charges", r.LogNormal(180*time.Millisecond, 600*time.Millisecond), 201)
		r.DB("INSERT INTO payments (id, cart_id, status, processor_ref, created_at) VALUES ($1, $2, $3, $4, now())", r.Ms(2, 5))
		r.DB("UPDATE carts SET status = 'paid' WHERE id = $1", r.Ms(1, 3))
		r.Gap(r.Ms(0.3, 0.8))
	})
	s.Background(api, seed.TypicalTraffic(8))
	return api, checkout
}

// seedAlertSnooze: the card processor has an incident, so the checkout
// error-rate alert keeps paging. Distractor: a second rule (checkout p95)
// that must stay untouched.
func seedAlertSnooze(s *seed.Scenario) error {
	p := s.Project("payments", seed.OTel)
	_, checkout := checkoutTraffic(s, p, s.Ago(40*time.Minute), 0.35)
	// Enough requests in the rule's 5-minute lookback even at the diurnal
	// trough, so the page always opens.
	checkout.Traffic(seed.Daily(3600))

	paging := p.PagingChannel("Payments on-call")
	email := p.EmailChannel("Payments team email")
	errRule := p.Rule(seed.Rule{Name: "Checkout error rate above 5%", Type: "error_rate_threshold", Channel: paging, Cooldown: 5, Severity: "critical",
		Config: map[string]any{"thresholdPercent": 5, "lookbackMinutes": 5, "minRequests": 10}})
	latRule := p.Rule(seed.Rule{Name: "Checkout p95 above 3s", Type: "endpoint_p95_threshold", Channel: email, Cooldown: 30,
		Config: map[string]any{"endpoint": "POST /api/checkout", "thresholdMs": 3000, "lookbackMinutes": 10}})
	s.Expose("ruleId", errRule)
	s.Expose("otherRuleId", latRule)
	s.AfterFlush(func() error {
		pg, err := p.WaitPage(errRule, 90*time.Second)
		if err != nil {
			return err
		}
		s.Expose("pageId", pg.ID)
		return nil
	})
	return nil
}
