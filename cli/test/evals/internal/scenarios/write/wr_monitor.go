package write

import (
	"fmt"
	"net/http"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/env"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("write/monitor-confirm-fix", wrSeedMonitorConfirmFix) }

// wrIncidentID reads the id of a monitor's newest open incident.
func wrIncidentID(st seed.CheckState) (int, string, error) {
	for _, inc := range st.Incidents {
		if inc["resolvedAt"] != nil {
			continue
		}
		id, ok := inc["id"].(float64)
		if !ok {
			continue
		}
		started, _ := inc["startedAt"].(string)
		return int(id), started, nil
	}
	return 0, "", fmt.Errorf("monitor has no open incident: %v", st.Incidents)
}

// wrSettleSchedule waits until the scheduler has run a new monitor's first
// due run (it enqueues a monitor right after creation and then moves
// nextRunAt one interval ahead) and that run has finished. Switching the
// stub before that would let the scheduled run see the new state.
func wrSettleSchedule(s *seed.Scenario, p *seed.Project, id int) error {
	if s.Sink.Dry() {
		return nil
	}
	var out struct {
		Check struct {
			IntervalSeconds int        `json:"intervalSeconds"`
			NextRunAt       time.Time  `json:"nextRunAt"`
			LastRunAt       *time.Time `json:"lastRunAt"`
			CreatedAt       time.Time  `json:"createdAt"`
		} `json:"check"`
	}
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		res, err := s.Sink.Admin(http.MethodGet, fmt.Sprintf("/api/synthetics/checks/%d", id), env.ProjectQuery(p.ID), nil)
		if err != nil {
			return err
		}
		if err := res.JSON(&out); err != nil {
			return err
		}
		c := out.Check
		interval := time.Duration(c.IntervalSeconds) * time.Second
		enqueued := c.NextRunAt.Add(-interval)
		if c.NextRunAt.After(c.CreatedAt.Add(interval/2)) && c.LastRunAt != nil && !c.LastRunAt.Before(enqueued) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("the scheduler never ran monitor %d", id)
}

// wrSeedMonitorConfirmFix: the card processor failed a share of charges,
// so the checkout health monitor (which pings the processor) went down and
// opened an incident. The processor has fixed its side: the stub is healthy
// again by the time the agent starts, but the monitor's next scheduled run
// is five minutes out, so it stays down until the agent runs it. The
// GatewayError issue must not be archived without the user's consent; a
// healthy homepage monitor is there to be left alone.
func wrSeedMonitorConfirmFix(s *seed.Scenario) error {
	p := s.Project("payments", seed.OTel)
	checkoutTraffic(s, p, s.Ago(50*time.Minute), 0.3)
	stub := s.Sink.Stub()
	base := wrStubURL(s)

	home := p.Check(seed.Check{Name: "Storefront homepage", URL: base + "/", IntervalSeconds: 300})
	chk := p.Check(seed.Check{Name: "Checkout API health", URL: base + "/api/checkout/health", IntervalSeconds: 300})
	for _, id := range []int{home, chk} {
		if err := wrSettleSchedule(s, p, id); err != nil {
			return err
		}
	}
	stub.SetHealthy(false)
	st, err := p.RunCheckUntil(chk, "down", 60*time.Second)
	if err != nil {
		return err
	}
	incident, started, err := wrIncidentID(st)
	if err != nil {
		return err
	}
	s.Expose("checkId", chk)
	s.Expose("homeCheckId", home)
	s.Expose("incidentId", incident)
	s.Expose("incidentStartedAt", started)

	s.AfterFlush(func() error {
		stub.SetHealthy(true)
		hash, err := p.GroupMatching(24*time.Hour, "upstream returned 502")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		return nil
	})
	return nil
}
