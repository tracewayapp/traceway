package debug

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/py-keyerror-webhook", dbgSeedPyKeyErrorWebhook) }

const dbgStarletteFrames = `Traceback (most recent call last):
  File "/usr/local/lib/python3.12/site-packages/opentelemetry/instrumentation/fastapi/__init__.py", line 307, in __call__
    await self.app(scope, receive, send)
  File "/app/app/access_log.py", line 27, in __call__
    await self.app(scope, receive, send_status)
  File "/usr/local/lib/python3.12/site-packages/starlette/middleware/exceptions.py", line 63, in __call__
    await wrap_app_handling_exceptions(self.app, conn)(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/starlette/_exception_handler.py", line 53, in wrapped_app
    raise exc
  File "/usr/local/lib/python3.12/site-packages/starlette/_exception_handler.py", line 42, in wrapped_app
    await app(scope, receive, sender)
  File "/usr/local/lib/python3.12/site-packages/starlette/routing.py", line 716, in __call__
    await self.middleware_stack(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/starlette/routing.py", line 736, in app
    await route.handle(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/starlette/routing.py", line 290, in handle
    await self.app(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/starlette/routing.py", line 78, in app
    await wrap_app_handling_exceptions(app, request)(scope, receive, send)
  File "/usr/local/lib/python3.12/site-packages/starlette/_exception_handler.py", line 53, in wrapped_app
    raise exc
  File "/usr/local/lib/python3.12/site-packages/starlette/_exception_handler.py", line 42, in wrapped_app
    await app(scope, receive, sender)
  File "/usr/local/lib/python3.12/site-packages/starlette/routing.py", line 75, in app
    response = await f(request)
               ^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/fastapi/routing.py", line 302, in app
    raw_response = await run_endpoint_function(
                   ^^^^^^^^^^^^^^^^^^^^^^^^^^^^
`

// dbgSeedPyKeyErrorWebhook: v2.6.0 (22h ago) links paid invoices to the
// customer's account and reads invoice["customer"], which the provider
// omits for guest payments: those deliveries answer 500 and the payment is
// never recorded. Distractor: a sender with a stale secret has been
// failing signature checks (401, WARN logs, every event type) for 3 days.
func dbgSeedPyKeyErrorWebhook(s *seed.Scenario) error {
	deployAt := s.Ago(22 * time.Hour).Truncate(time.Minute).Add(-6 * time.Minute)
	staleFrom := s.Ago(3*24*time.Hour + 5*time.Hour).Truncate(time.Minute)
	p := s.Project("payments", seed.OTel)
	api := p.Service("payments-webhooks", seed.Python("fastapi", "psycopg")).Versions("2.5.3", seed.Deploy{At: deployAt, Version: "2.6.0"})
	stack := dbgStarletteFrames + `  File "/usr/local/lib/python3.12/site-packages/fastapi/routing.py", line 213, in run_endpoint_function
    return await dependant.call(**values)
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/app/app/main.py", line 36, in payments_webhook
    result = handle_event(event, store)
             ^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/app/app/webhooks.py", line 37, in handle_event
    return handler(event["data"]["object"], store)
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/app/app/webhooks.py", line 41, in _invoice_paid
    customer_id = invoice["customer"]
                  ~~~~~~~^^^^^^^^^^^^
KeyError: 'customer'
`
	other := []string{"invoice.created", "invoice.finalized", "customer.updated", "charge.succeeded", "payment_intent.succeeded"}

	hook := api.Endpoint("POST", "/api/webhooks/payments")
	hook.Traffic(seed.Daily(1600), seed.Diurnal(), seed.Over(7*24*time.Hour))
	hook.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.03, Max: 25})
	hook.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.6, 1.6)) // body read, signature check
		if !r.Start().Before(staleFrom) && r.Chance(0.02) {
			r.Log(seed.SevWarn, "webhook signature verification failed: signature mismatch")
			r.Status(401)
			r.Gap(r.Ms(0.2, 0.5))
			return
		}
		evID := fmt.Sprintf("evt_3P%07x%08x", r.IntN(1<<28), r.IntN(1<<31))
		x := r.Float()
		switch {
		case x < 0.45:
			r.RootAttr("payment.event.id", evID)
			r.RootAttr("payment.event.type", "invoice.paid")
			guest := r.Chance(0.08)
			if r.Version() == "2.6.0" {
				if guest {
					r.Error(seed.Exc{Type: "KeyError", Message: "'customer'", Stack: stack, Escaped: true})
					r.Status(500)
					r.Gap(r.Ms(0.3, 0.8))
					return
				}
				r.DB("SELECT id, customer_id FROM accounts WHERE customer_id = %s", r.Ms(1, 3), seed.Rows(1))
			}
			r.DB("INSERT INTO payments (invoice_id, amount_cents, currency, account_id, paid_at) VALUES (%s, %s, %s, %s, now()) ON CONFLICT (invoice_id) DO NOTHING", r.Ms(2, 5))
			if r.Version() == "2.6.0" && r.Chance(0.6) {
				r.DB("UPDATE subscriptions SET status = 'active', updated_at = now() WHERE account_id = %s AND provider_id = %s", r.Ms(1, 4))
			}
		case x < 0.5:
			r.RootAttr("payment.event.id", evID)
			r.RootAttr("payment.event.type", "customer.subscription.deleted")
			r.DB("UPDATE subscriptions SET status = 'cancelled', updated_at = now() WHERE provider_id = %s", r.Ms(1, 4))
		default:
			r.RootAttr("payment.event.id", evID)
			r.RootAttr("payment.event.type", other[r.IntN(len(other))])
		}
		r.Gap(r.Ms(0.2, 0.6))
	})

	status := api.Endpoint("GET", "/api/payments/{invoice_id}")
	status.Traffic(seed.Daily(250), seed.Diurnal(), seed.Over(7*24*time.Hour))
	status.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.0))
		r.DB("SELECT invoice_id, amount_cents, currency, account_id, paid_at FROM payments WHERE invoice_id = %s", r.Ms(1, 4))
		r.Gap(r.Ms(0.3, 0.8))
	})
	s.Background(api, seed.TypicalTraffic(4))
	api.Host().CPU(seed.Flat(0.14, 0.26))

	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(8*24*time.Hour, "_invoice_paid")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		return nil
	})
	return nil
}
