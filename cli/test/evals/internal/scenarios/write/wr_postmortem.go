package write

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() {
	scenarios.Register("write/postmortem-create", wrSeedPostMortemCreate)
	scenarios.Register("write/postmortem-conflict", wrSeedPostMortemConflict)
}

// wrOlderPostMortem is a document the team wrote for an earlier incident.
const wrOlderPostMortem = `## Summary
Between 14:05 and 14:41 UTC on 30 September about 9% of checkout attempts failed because charge requests to the card processor timed out.

## Impact
312 checkout attempts failed with a 504. No customer was charged twice.

## Timeline (UTC)
- 14:05 First charge timeouts.
- 14:09 Checkout error-rate page opened; acknowledged at 14:12.
- 14:30 The processor confirmed degraded capacity in one region.
- 14:41 Timeouts stopped.

## What went wrong
The processor's eu-west region was overloaded; our client waits 10 s per charge and has no fallback region.

## Resolution
The processor shifted traffic to another region.

## Follow-up actions
- Lower the charge timeout to 4 s and retry once against the secondary region (owner: Sam).
`

const wrReadOnlyStack = "goroutine 733 [running]:\n" +
	"example.com/payments-api/internal/store.(*Store).CreatePendingPayment(0xc000210a80, {0x1c8e2a0, 0xc00041e0c0}, 0xc0004f6000)\n\t/app/internal/store/payments.go:31 +0x2c4\n" +
	"example.com/payments-api/internal/checkout.(*Service).Pay(0xc000182a50, {0x1c8e2a0, 0xc00041e0c0}, 0xc0004a4180)\n\t/app/internal/checkout/pay.go:47 +0x1a9"

// wrSeedPostMortemCreate: payments-api 3.5.0 pointed its database pool at
// the read replica, so every checkout write failed until the rollback to
// 3.4.1. The checkout health monitor caught it (incident N), the
// error-rate rule paged on-call, the owner acknowledged it and resolved it
// after the recovery. The agent writes the post-mortem for incident N from
// that telemetry, shows the draft, and creates it after approval.
//
// The monitor stamps the incident with real probe times, so the outage is
// laid out around the moment the monitor first fails (t0): the bad deploy
// 40 s before it, the rollback 90 s after it, the recovery run 25 s after
// the rollback. The scenario's now is moved past that and the seed waits
// it out (about two and a half minutes).
func wrSeedPostMortemCreate(s *seed.Scenario) error {
	p := s.Project("payments", seed.OTel)
	older, _ := p.PostMortem("Checkout failures: card processor timeouts (Sep 30)", wrOlderPostMortem, []string{"checkout", "processor"}, 0)
	s.Expose("existingPostMortems", []int{older})

	paging := p.PagingChannel("Payments on-call")
	// A 5-minute cooldown would let the rule fire again while its lookback
	// still holds the failures, opening a second page after the owner
	// resolved the first, in the middle of the agent's run.
	rule := p.Rule(seed.Rule{Name: "Checkout error rate above 5%", Type: "error_rate_threshold", Channel: paging, Cooldown: 60, Severity: "critical",
		Config: map[string]any{"thresholdPercent": 5, "lookbackMinutes": 5, "minRequests": 10}})

	stub := s.Sink.Stub()
	chk := p.Check(seed.Check{Name: "Checkout API health", URL: wrStubURL(s) + "/api/checkout/health", IntervalSeconds: 300})
	if err := wrSettleSchedule(s, p, chk); err != nil {
		return err
	}
	stub.SetHealthy(false)
	st, err := p.RunCheckUntil(chk, "down", 60*time.Second)
	if err != nil {
		return err
	}
	incident, startedStr, err := wrIncidentID(st)
	if err != nil {
		return err
	}
	t0 := wrClock(s)
	if t, err := time.Parse(time.RFC3339Nano, startedStr); err == nil && !s.Sink.Dry() {
		t0 = t.UTC()
	}
	badDeploy := t0.Add(-40 * time.Second)
	rollback := t0.Add(90 * time.Second)
	recoverAt := rollback.Add(25 * time.Second)
	end := recoverAt.Add(20 * time.Second)
	wrEndTelemetryAt(s, end)

	api := p.Service("payments-api", seed.Go("chi", "pgx")).Versions("3.4.1",
		seed.Deploy{At: badDeploy, Version: "3.5.0"}, seed.Deploy{At: rollback, Version: "3.4.1"})
	var failed int
	checkout := api.Endpoint("POST", "/api/checkout")
	checkout.Traffic(seed.Daily(24000), seed.Diurnal(), seed.Over(6*time.Hour))
	checkout.Detail(seed.DetailPolicy{Recent: 30 * time.Minute, Rate: 0.04, Max: 20})
	checkout.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.5, 1.5))
		host := seed.A("server.address", "payments-db.internal")
		if r.Version() == "3.5.0" {
			host = seed.A("server.address", "payments-db-ro.internal")
		}
		r.DB("SELECT id, user_id, total_cents, currency FROM carts WHERE id = $1", r.Ms(1, 4), host)
		if r.Version() == "3.5.0" {
			failed++
			ins := r.DB("INSERT INTO payments (id, cart_id, status, created_at) VALUES ($1, $2, 'pending', now())", r.Ms(1, 3), host)
			ins.Status, ins.StatusMsg = seed.StatusError, "ERROR: cannot execute INSERT in a read-only transaction (SQLSTATE 25006)"
			r.Error(seed.Exc{Type: "*pgconn.PgError", Message: ins.StatusMsg, Stack: wrReadOnlyStack})
			r.Log(seed.SevError, "checkout failed", seed.A("error", ins.StatusMsg))
			r.Status(500)
			r.Gap(r.Ms(0.2, 0.6))
			return
		}
		r.DB("INSERT INTO payments (id, cart_id, status, created_at) VALUES ($1, $2, 'pending', now())", r.Ms(2, 5))
		r.HTTP("POST", "https://api.cardprocessor.example/v2/charges", r.LogNormal(180*time.Millisecond, 600*time.Millisecond), 201)
		r.DB("UPDATE payments SET status = 'captured', processor_ref = $2 WHERE id = $1", r.Ms(1, 3))
		r.Gap(r.Ms(0.3, 0.8))
	})
	get := api.Endpoint("GET", "/api/payments/{id}")
	get.Traffic(seed.Daily(9000), seed.Diurnal(), seed.Over(6*time.Hour))
	get.Detail(seed.DetailPolicy{Recent: 30 * time.Minute, Rate: 0.01, Max: 5})
	get.Handler(func(r *seed.Req) {
		r.Gap(r.Ms(0.3, 0.9))
		r.DB("SELECT id, cart_id, status, processor_ref, created_at FROM payments WHERE id = $1", r.Ms(1, 3))
		r.Compute("encode response", r.Ms(0.2, 0.8))
	})
	api.Log(badDeploy.Add(2*time.Second), seed.SevInfo, "database pool ready", seed.A("db.host", "payments-db-ro.internal"), seed.A("pool.max_conns", 20))
	api.Log(rollback.Add(2*time.Second), seed.SevInfo, "database pool ready", seed.A("db.host", "payments-db.internal"), seed.A("pool.max_conns", 20))
	s.Background(api, seed.TrafficOpts{Endpoints: 6, PerDay: 150, NoErrors: true})

	s.Expose("checkId", chk)
	s.Expose("incidentId", incident)
	s.Expose("incidentStartedAt", t0.Format(time.RFC3339))
	s.Expose("badDeployAt", badDeploy.Format(time.RFC3339))
	s.Expose("rollbackAt", rollback.Format(time.RFC3339))

	s.AfterFlush(func() error {
		pg, err := p.WaitPage(rule, 90*time.Second)
		if err != nil {
			return err
		}
		wrSleepUntil(s, t0.Add(35*time.Second))
		if err := wrAdminPost(s, fmt.Sprintf("/api/pages/%d/acknowledge", pg.ID), p.ID); err != nil {
			return err
		}
		wrSleepUntil(s, recoverAt)
		stub.SetHealthy(true)
		up, err := p.RunCheckUntil(chk, "up", 60*time.Second)
		if err != nil {
			return err
		}
		resolved := wrClock(s)
		for _, inc := range up.Incidents {
			if id, _ := inc["id"].(float64); int(id) == incident {
				if r, ok := inc["resolvedAt"].(string); ok {
					if t, err := time.Parse(time.RFC3339Nano, r); err == nil {
						resolved = t.UTC()
					}
				}
			}
		}
		wrSleepUntil(s, resolved.Add(10*time.Second))
		if err := wrAdminPost(s, fmt.Sprintf("/api/pages/%d/resolve", pg.ID), p.ID); err != nil {
			return err
		}
		hash, err := p.GroupMatching(time.Hour, "read-only transaction")
		if err != nil {
			return err
		}
		s.Expose("pageId", pg.ID)
		s.Expose("pageOpenedAt", pg.CreatedAt)
		s.Expose("incidentResolvedAt", resolved.Format(time.RFC3339))
		s.Expose("outageSeconds", int(resolved.Sub(t0).Seconds()))
		s.Expose("failedCheckouts", failed)
		s.Expose("hash", hash)
		wrSleepUntil(s, end.Add(3*time.Second))
		return nil
	})
	return nil
}

// wrAdminPost does what the owner does in the dashboard (acknowledge or
// resolve a page).
func wrAdminPost(s *seed.Scenario, path, projectID string) error {
	res, err := s.Sink.Admin(http.MethodPost, path, env.ProjectQuery(projectID), map[string]any{})
	if err != nil {
		return err
	}
	return res.Err("POST " + path)
}

// wrConflictPostMortem is the draft the user asks the agent to extend.
const wrConflictPostMortem = `## Summary
On 16 October between 13:58 and 14:47 UTC about 11% of checkout attempts failed with 502s from the card processor.

## Impact
About 540 checkout attempts failed. Customers could retry; no double charges were found.

## Timeline (UTC)
- 13:58 First 502s from the processor.
- 14:03 Checkout error-rate page opened; acknowledged at 14:06.
- 14:47 502s stopped.

## Resolution
The processor rolled back a faulty load balancer change on their side.

## Follow-up actions
- Ask the processor for their incident report (owner: Priya).
`

const (
	wrTeammateLine = "- 14:52 Priya confirmed with the processor's support that their incident INC-20931 covered our merchant account.\n"
	wrAgentLine    = "- Add a circuit breaker around the card processor client so checkout fails fast while the processor is degraded (owner: Sam).\n"
)

// wrSeedPostMortemConflict: an existing post-mortem draft. The user asks the
// agent to add a follow-up action; right after the agent first reads it,
// a teammate adds a timeline entry (the case's hook). An update based on the
// stale read must answer 409, and the final document must have both edits.
func wrSeedPostMortemConflict(s *seed.Scenario) error {
	p := s.Project("payments", seed.OTel)
	checkoutTraffic(s, p, s.Ago(26*time.Hour), 0)
	id, _ := p.PostMortem("Checkout failures: card processor 502s (Oct 16)", wrConflictPostMortem, []string{"checkout", "processor"}, 0)
	const anchor = "- 14:47 502s stopped.\n"
	teammate := strings.Replace(wrConflictPostMortem, anchor, anchor+wrTeammateLine, 1)
	s.Expose("postMortemId", id)
	s.Expose("teammateAnchor", anchor)
	s.Expose("teammateLine", wrTeammateLine)
	s.Expose("staleContent", wrConflictPostMortem+wrAgentLine)
	s.Expose("finalContent", teammate+wrAgentLine)
	return nil
}
