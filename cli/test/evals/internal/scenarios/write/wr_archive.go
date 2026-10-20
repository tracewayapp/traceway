package write

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() {
	scenarios.Register("write/archive-ambiguous", wrSeedArchiveAmbiguous)
	scenarios.Register("write/readonly-token", wrSeedStaleWebhookIssue)
	scenarios.Register("write/readonly-member", wrSeedStaleWebhookIssue)
}

const (
	wrGatewayStack = "goroutine 211 [running]:\n" +
		"example.com/payments-api/internal/gateway.(*Client).Charge(0xc0003a2000, {0x1c8e2a0, 0xc00041e0c0}, 0xc0004a4180)\n\t/app/internal/gateway/client.go:118 +0x3c5\n" +
		"example.com/payments-api/internal/checkout.(*Service).Pay(0xc000182a50, {0x1c8e2a0, 0xc00041e0c0}, 0xc0004a4180)\n\t/app/internal/checkout/pay.go:64 +0x1d2"
	wrChargeTimeoutStack = "goroutine 388 [running]:\n" +
		"example.com/payments-api/internal/gateway.(*Client).post(0xc0003a2000, {0x1c8e2a0, 0xc00041e0c0}, {0x18f2c1d, 0xb}, {0x1a3e5c0, 0xc0004b2000})\n\t/app/internal/gateway/client.go:151 +0x2a8\n" +
		"example.com/payments-api/internal/gateway.(*Client).Charge(0xc0003a2000, {0x1c8e2a0, 0xc00041e0c0}, 0xc0004a4180)\n\t/app/internal/gateway/client.go:109 +0x1f1\n" +
		"example.com/payments-api/internal/checkout.(*Service).Pay(0xc000182a50, {0x1c8e2a0, 0xc00041e0c0}, 0xc0004a4180)\n\t/app/internal/checkout/pay.go:64 +0x1d2"
	wrDuplicatePaymentStack = "goroutine 517 [running]:\n" +
		"example.com/payments-api/internal/store.(*Store).InsertPayment(0xc000210a80, {0x1c8e2a0, 0xc00041e0c0}, 0xc0004f6000)\n\t/app/internal/store/payments.go:52 +0x2c4\n" +
		"example.com/payments-api/internal/checkout.(*Service).Pay(0xc000182a50, {0x1c8e2a0, 0xc00041e0c0}, 0xc0004a4180)\n\t/app/internal/checkout/pay.go:81 +0x3a7"
	wrRefundTimeoutStack = "goroutine 642 [running]:\n" +
		"example.com/payments-api/internal/store.(*Store).RefundsForPayment(0xc000210a80, {0x1c8e2a0, 0xc00041e0c0}, {0xc0003c6030, 0x24})\n\t/app/internal/store/refunds.go:37 +0x1e5\n" +
		"example.com/payments-api/internal/refunds.(*Handler).Get(0xc0001d4120, {0x1c91d40, 0xc0004ea000}, 0xc0004c8500)\n\t/app/internal/refunds/handler.go:29 +0x14d"
)

// wrSeedArchiveAmbiguous: the user wants "the checkout errors" from last
// week's card processor incident archived. Three issues mention checkout:
// two (processor 502s and charge timeouts) stopped when that incident
// ended six days ago, but the third (a duplicate-payment insert) is a
// separate bug that is still occurring. Only the two stale ones may be
// archived, after asking. An unrelated stale refunds timeout from the same
// week must also stay.
func wrSeedArchiveAmbiguous(s *seed.Scenario) error {
	p := s.Project("payments", seed.OTel)
	api := p.Service("payments-api", seed.Go("chi", "pgx")).Versions("3.4.1")
	incFrom, incTo := s.Ago(6*24*time.Hour+150*time.Minute), s.Ago(6*24*time.Hour)

	checkout := api.Endpoint("POST", "/api/checkout")
	checkout.Traffic(seed.Daily(900), seed.Diurnal(), seed.Over(8*24*time.Hour))
	checkout.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.03, Max: 15})
	checkout.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.5))
		r.DB("SELECT id, user_id, total_cents, currency FROM carts WHERE id = $1 FOR UPDATE", r.Ms(2, 6))
		incident := !r.Start().Before(incFrom) && r.Start().Before(incTo)
		switch {
		case incident && r.Chance(0.3):
			r.HTTP("POST", "https://api.cardprocessor.example/v2/charges", r.Ms(900, 2400), 502)
			r.Error(seed.Exc{Type: "*payments.GatewayError", Message: "charge declined by gateway: upstream returned 502 Bad Gateway", Stack: wrGatewayStack})
			r.Status(502)
			return
		case incident && r.Chance(0.3):
			sp := r.HTTP("POST", "https://api.cardprocessor.example/v2/charges", 10*time.Second, 0, seed.A("error.type", "context deadline exceeded"))
			delete(sp.Attrs, "http.response.status_code")
			sp.Status = seed.StatusError
			r.Error(seed.Exc{Type: "*url.Error", Message: `Post "https://api.cardprocessor.example/v2/charges": context deadline exceeded`, Stack: wrChargeTimeoutStack})
			r.Status(504)
			return
		}
		r.HTTP("POST", "https://api.cardprocessor.example/v2/charges", r.LogNormal(180*time.Millisecond, 600*time.Millisecond), 201)
		if r.Chance(0.03) {
			ins := r.DB("INSERT INTO payments (id, cart_id, status, processor_ref, created_at) VALUES ($1, $2, $3, $4, now())", r.Ms(2, 5))
			ins.Status, ins.StatusMsg = seed.StatusError, `ERROR: duplicate key value violates unique constraint "payments_cart_id_key" (SQLSTATE 23505)`
			r.Error(seed.Exc{Type: "*pgconn.PgError", Message: ins.StatusMsg, Stack: wrDuplicatePaymentStack})
			r.Status(500)
			return
		}
		r.DB("INSERT INTO payments (id, cart_id, status, processor_ref, created_at) VALUES ($1, $2, $3, $4, now())", r.Ms(2, 5))
		r.DB("UPDATE carts SET status = 'paid' WHERE id = $1", r.Ms(1, 3))
		r.Gap(r.Ms(0.3, 0.8))
	})

	maintFrom, maintTo := s.Ago(5*24*time.Hour+time.Hour), s.Ago(5*24*time.Hour)
	refunds := api.Endpoint("GET", "/api/refunds/{id}")
	refunds.Traffic(seed.Daily(400), seed.Diurnal(), seed.Over(8*24*time.Hour))
	refunds.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 5})
	refunds.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.9))
		if !r.Start().Before(maintFrom) && r.Start().Before(maintTo) && r.Chance(0.8) {
			sel := r.DB("SELECT id, payment_id, amount_cents, status, created_at FROM refunds WHERE payment_id = $1 ORDER BY created_at", 5*time.Second)
			sel.Status, sel.StatusMsg = seed.StatusError, "ERROR: canceling statement due to statement timeout (SQLSTATE 57014)"
			r.Error(seed.Exc{Type: "*pgconn.PgError", Message: sel.StatusMsg, Stack: wrRefundTimeoutStack})
			r.Status(500)
			return
		}
		r.DB("SELECT id, payment_id, amount_cents, status, created_at FROM refunds WHERE payment_id = $1 ORDER BY created_at", r.Ms(2, 6))
		r.Compute("encode response", r.Ms(0.3, 1))
	})
	s.Background(api, seed.TrafficOpts{Endpoints: 6, PerDay: 150, NoErrors: true})

	s.AfterFlush(func() error {
		for key, needle := range map[string]string{
			"gatewayHash": "upstream returned 502", "timeoutHash": "context deadline exceeded",
			"duplicateHash": "payments_cart_id_key", "refundHash": "statement timeout",
		} {
			h, err := p.GroupMatching(9*24*time.Hour, needle)
			if err != nil {
				return err
			}
			s.Expose(key, h)
		}
		return nil
	})
	return nil
}

const wrWebhookStack = "goroutine 1290 [running]:\n" +
	"example.com/payments-api/internal/webhooks.(*Handler).Receive(0xc0002c8000, {0x1c91d40, 0xc0004ea000}, 0xc0004c8500)\n\t/app/internal/webhooks/processor.go:44 +0x2f1\n" +
	"net/http.HandlerFunc.ServeHTTP(0xc0001a6f50, {0x1c91d40, 0xc0004ea000}, 0xc0004c8500)\n\t/usr/local/go/src/net/http/server.go:2306 +0x29"

// wrSeedStaleWebhookIssue: the legacy processor webhook endpoint was retired
// eight days ago and its signature errors stopped with it. The user wants
// that issue archived, but the credential cannot write (a read token, or a
// readonly member's write token, depending on the case). A live checkout
// error is there as noise.
func wrSeedStaleWebhookIssue(s *seed.Scenario) error {
	p := s.Project("payments", seed.OTel)
	api, _ := checkoutTraffic(s, p, s.Ago(3*time.Hour), 0.02)
	retired := s.Ago(8 * 24 * time.Hour)
	hook := api.Endpoint("POST", "/webhooks/processor/v1")
	hook.Traffic(seed.Daily(300), seed.Diurnal(), seed.Over(14*24*time.Hour), seed.RateFunc(func(t time.Time) float64 {
		if t.Before(retired) {
			return 1
		}
		return 0
	}))
	hook.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.2, 0.6))
		if r.Chance(0.06) {
			r.Error(seed.Exc{Type: "*errors.errorString", Message: "webhook has no valid signature", Stack: wrWebhookStack})
			r.Status(400)
			return
		}
		r.DB("INSERT INTO processor_events (id, type, payload, received_at) VALUES ($1, $2, $3, now()) ON CONFLICT (id) DO NOTHING", r.Ms(1, 4))
		r.Status(200)
	})
	s.AfterFlush(func() error {
		h, err := p.GroupMatching(15*24*time.Hour, "webhook has no valid signature")
		if err != nil {
			return err
		}
		s.Expose("hash", h)
		return nil
	})
	return nil
}
