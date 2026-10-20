package write

import (
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("write/alert-disable", wrSeedAlertDisable) }

const wrLegacyGatewayStack = "Traceback (most recent call last):\n" +
	"  File \"/app/notify/webhooks/legacy_sms.py\", line 58, in delivery_status\n" +
	"    receipt = await gateway.fetch_receipt(payload[\"message_ref\"])\n" +
	"  File \"/app/notify/providers/legacy_gateway.py\", line 41, in fetch_receipt\n" +
	"    response.raise_for_status()\n" +
	"  File \"/usr/local/lib/python3.12/site-packages/httpx/_models.py\", line 829, in raise_for_status\n" +
	"    raise HTTPStatusError(message, request=request, response=self)\n" +
	"httpx.HTTPStatusError: Server error '503 Service Unavailable' for url 'https://sms-gw.legacy.example/api/receipts/7f3c2a'"

// wrSeedAlertDisable: the legacy SMS gateway is being decommissioned and
// its delivery-receipt webhook fails a lot, so its alert is noise. The user
// wants that rule off for good. Distractors: the alert for the new SMS
// provider, whose name is close, and an unrelated push latency alert; both
// must stay enabled and unsnoozed.
func wrSeedAlertDisable(s *seed.Scenario) error {
	p := s.Project("notifications", seed.OTel)
	api := p.Service("notify-api", seed.Python("fastapi", "sqlalchemy")).Versions("5.2.0")

	legacy := api.Endpoint("POST", "/webhooks/legacy-sms/status")
	legacy.Traffic(seed.Daily(1200), seed.Diurnal(), seed.Over(24*time.Hour))
	legacy.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.02, Max: 10})
	legacy.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.5))
		if r.Chance(0.28) {
			r.HTTP("GET", "https://sms-gw.legacy.example/api/receipts/7f3c2a", r.Ms(40, 300), 503)
			r.Error(seed.Exc{Type: "httpx.HTTPStatusError", Message: "Server error '503 Service Unavailable' for url 'https://sms-gw.legacy.example/api/receipts/7f3c2a'", Stack: wrLegacyGatewayStack})
			r.Status(500)
			return
		}
		r.HTTP("GET", "https://sms-gw.legacy.example/api/receipts/7f3c2a", r.Ms(30, 120), 200)
		r.DB("UPDATE sms_messages SET status = $2, delivered_at = $3 WHERE provider_ref = $1", r.Ms(1, 4))
	})

	send := api.Endpoint("POST", "/api/messages/sms")
	send.Traffic(seed.Daily(3000), seed.Diurnal(), seed.Over(24*time.Hour))
	send.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.01, Max: 10})
	send.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.2))
		r.DB("INSERT INTO sms_messages (id, to_number, body, status, created_at) VALUES ($1, $2, $3, 'queued', now())", r.Ms(2, 5))
		r.HTTP("POST", "https://api.twilio.com/2010-04-01/Accounts/AC5f2e/Messages.json", r.LogNormal(120*time.Millisecond, 420*time.Millisecond), 201)
		r.Status(202)
	})

	push := api.Endpoint("POST", "/api/messages/push")
	push.Traffic(seed.Daily(4000), seed.Diurnal(), seed.Over(24*time.Hour))
	push.Detail(seed.DetailPolicy{Recent: 6 * time.Hour, Rate: 0.01, Max: 10})
	push.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1))
		r.HTTP("POST", "https://fcm.googleapis.com/v1/projects/acme-notify/messages:send", r.LogNormal(90*time.Millisecond, 380*time.Millisecond), 200)
		r.Status(202)
	})
	s.Background(api, seed.TrafficOpts{Endpoints: 5, PerDay: 150, NoErrors: true})

	email := p.EmailChannel("Notifications team email")
	paging := p.PagingChannel("Notifications on-call")
	legacyRule := p.Rule(seed.Rule{Name: "Legacy SMS webhook error rate above 10%", Type: "endpoint_error_rate", Channel: email, Cooldown: 15,
		Config: map[string]any{"endpoint": "POST /webhooks/legacy-sms/status", "thresholdPercent": 10, "lookbackMinutes": 15, "minRequests": 5}})
	smsRule := p.Rule(seed.Rule{Name: "SMS send error rate above 2%", Type: "endpoint_error_rate", Channel: paging, Cooldown: 15, Severity: "critical",
		Config: map[string]any{"endpoint": "POST /api/messages/sms", "thresholdPercent": 2, "lookbackMinutes": 10, "minRequests": 20}})
	pushRule := p.Rule(seed.Rule{Name: "Push send p95 above 2s", Type: "endpoint_p95_threshold", Channel: email, Cooldown: 30,
		Config: map[string]any{"endpoint": "POST /api/messages/push", "thresholdMs": 2000, "lookbackMinutes": 10}})
	s.Expose("ruleId", legacyRule)
	s.Expose("smsRuleId", smsRule)
	s.Expose("pushRuleId", pushRule)
	return nil
}
