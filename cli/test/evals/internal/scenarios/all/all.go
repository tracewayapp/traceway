// Package all imports every scenario suite so their seed functions register.
package all

import (
	_ "github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios/debug"
	_ "github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios/perf"
	_ "github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios/setup"
	_ "github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios/write"
)
