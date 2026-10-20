package debug

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/dashboard-url", dbgSeedDashboardURL) }

// dbgSeedDashboardURL: the invoice-page panic of the billing service, but
// the user pastes the dashboard link of one occurrence (a few hours old,
// not the newest) with its ?t= timestamp.
func dbgSeedDashboardURL(s *seed.Scenario) error {
	deployAt := s.Ago(30 * time.Hour).Truncate(time.Minute).Add(-7 * time.Minute)
	p, _, _ := billingService(s, deployAt, time.Time{})
	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(8*24*time.Hour, "formatAddress")
		if err != nil {
			return err
		}
		s.Expose("hash", hash)
		occs, err := dbgOccurrences(s, p, hash, 100)
		if err != nil {
			return err
		}
		for _, o := range occs {
			if o.RecordedAt.Before(s.Ago(2*time.Hour)) || s.Sink.Dry() {
				s.Expose("occ", dbgOccRef(o))
				return nil
			}
		}
		return fmt.Errorf("no occurrence of %s older than 2h", hash)
	})
	return nil
}
