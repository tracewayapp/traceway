// Package setup holds the seed functions of the setup suite. Setup seeds
// create the Traceway projects a case starts from and nothing else: the
// telemetry has to come from the app the agent instruments.
package setup

import (
	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() {
	scenarios.Register("setup/go-nethttp", func(s *seed.Scenario) error {
		s.Project("hello-api", seed.OTel)
		return nil
	})
}
