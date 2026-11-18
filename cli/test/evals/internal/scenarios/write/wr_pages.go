package write

import (
	"fmt"
	"net/http"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() {
	scenarios.Register("write/page-ack", wrSeedPageAck)
	scenarios.Register("write/investigate-only", wrSeedInvestigateOnly)
	scenarios.Register("write/optin-off", wrSeedOptinOff)
}

// wrCheckoutPage is the payments service whose card processor started
// failing a share of charges, with the checkout error-rate rule paging
// on-call. It exposes the page once the rule has fired.
func wrCheckoutPage(s *seed.Scenario, failingFor time.Duration, failShare float64) (*seed.Project, *seed.Service, int) {
	p := s.Project("payments", seed.OTel)
	api, checkout := checkoutTraffic(s, p, s.Ago(failingFor), failShare)
	// More checkout traffic than the shared helper's, so the rule's 5-minute
	// window (of which only 3-4 minutes hold data when it is evaluated)
	// clears minRequests even at the diurnal trough.
	checkout.Traffic(seed.Daily(5400))
	paging := wrTeamPaging(s, p, "Payments on-call", p.EmailChannel("Payments team email"))
	rule := p.Rule(seed.Rule{Name: "Checkout error rate above 5%", Type: "error_rate_threshold", Channel: paging, Cooldown: 5, Severity: "critical",
		Config: map[string]any{"thresholdPercent": 5, "lookbackMinutes": 5, "minRequests": 10}})
	s.AfterFlush(func() error {
		pg, err := p.WaitPage(rule, 90*time.Second)
		if err != nil {
			return err
		}
		s.Expose("pageId", pg.ID)
		return nil
	})
	return p, api, paging
}

// wrTeamPaging is a paging channel whose policy pages the owner and, when
// nobody acknowledges within 10 minutes, escalates to the team's channel,
// so an acknowledgement visibly stops an escalation that is still ahead.
func wrTeamPaging(s *seed.Scenario, p *seed.Project, name string, team int) int {
	org, user := s.Sink.Org()
	path := fmt.Sprintf("/api/organizations/%d/escalation-policies", org)
	res, err := s.Sink.Admin(http.MethodPost, path, nil, map[string]any{
		"name": name + " policy",
		"definition": map[string]any{"schemaVersion": 1, "repeatCount": 0, "steps": []map[string]any{
			{"delayMinutes": 10, "targets": []map[string]any{{"type": "user", "id": user}}},
			{"delayMinutes": 10, "targets": []map[string]any{{"type": "channel", "id": team}}},
		}},
	})
	if err == nil {
		err = res.Err("POST " + path)
	}
	if err != nil {
		panic(err)
	}
	id, _ := res.Map()["id"].(float64)
	return p.Channel(name, "escalation", map[string]any{"policyId": int(id)})
}

// wrSeedPageAck: the checkout error-rate page is open, and so is an
// unrelated latency page for the payouts summary. The user is on the
// checkout one and wants it acknowledged; the payouts page must stay open
// and nothing may be resolved while the processor is still failing.
func wrSeedPageAck(s *seed.Scenario) error {
	p, api, paging := wrCheckoutPage(s, 25*time.Minute, 0.35)
	payouts := api.Endpoint("GET", "/api/payouts/summary")
	payouts.Traffic(seed.Daily(1440), seed.Over(24*time.Hour))
	payouts.Detail(seed.DetailPolicy{Recent: time.Hour, Rate: 0.1, Max: 10})
	payouts.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.4, 1.2))
		r.DB("SELECT merchant_id, currency, SUM(amount_cents) FROM payout_items WHERE settled_at >= $1 GROUP BY merchant_id, currency", r.LogNormal(2600*time.Millisecond, 4200*time.Millisecond))
		r.Compute("encode response", r.Ms(1, 4))
	})
	latRule := p.Rule(seed.Rule{Name: "Payouts summary p95 above 2s", Type: "endpoint_p95_threshold", Channel: paging, Cooldown: 30, Severity: "warning",
		Config: map[string]any{"endpoint": "GET /api/payouts/summary", "thresholdMs": 2000, "lookbackMinutes": 15}})
	s.AfterFlush(func() error {
		pg, err := p.WaitPage(latRule, 90*time.Second)
		if err != nil {
			return err
		}
		s.Expose("otherPageId", pg.ID)
		return nil
	})
	return nil
}

// wrSeedInvestigateOnly: one open checkout page; the user only wants to
// know what is going on.
func wrSeedInvestigateOnly(s *seed.Scenario) error {
	wrCheckoutPage(s, 35*time.Minute, 0.35)
	return nil
}

// wrSeedOptinOff: the same page, in an organization whose Agent Access is
// off (the case sets agent_access: false).
func wrSeedOptinOff(s *seed.Scenario) error {
	wrCheckoutPage(s, 20*time.Minute, 0.35)
	return nil
}
