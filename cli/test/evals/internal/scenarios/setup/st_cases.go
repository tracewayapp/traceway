package setup

import (
	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

// The fast-path cases start from the one project the user's token belongs
// to; the plan-path case starts from an empty organization, because the
// approved plan is what creates its projects.
func init() {
	scenarios.Register("setup/node-express", func(s *seed.Scenario) error {
		s.Project("orders-api", seed.OTel)
		return nil
	})
	scenarios.Register("setup/python-fastapi", func(s *seed.Scenario) error {
		s.Project("inventory-api", seed.OTel)
		return nil
	})
	scenarios.Register("setup/nextjs", func(s *seed.Scenario) error {
		s.Project("catalog", seed.OTel)
		return nil
	})
	scenarios.Register("setup/react-browser", func(s *seed.Scenario) error {
		s.Project("shop-web", seed.Browser, "react")
		return nil
	})
	scenarios.Register("setup/monorepo-plan", func(s *seed.Scenario) error {
		return nil
	})
}
