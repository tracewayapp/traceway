// Package guard enforces the manual-only policy in code: paid
// commands start in CI only inside a hand-dispatched cli-evals.yml run behind
// the evals environment gate, locally only after a confirmation typed on a
// terminal, and always under a mandatory cost cap. The real OpenRouter client asks Live()
// before every request, and Live() is false inside `go test`.
package guard

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync/atomic"
	"testing"
)

// WorkflowFile is the only workflow allowed to run paid evals.
const WorkflowFile = ".github/workflows/cli-evals.yml"

var live atomic.Bool

// Live reports whether paid model calls are unlocked for this process. It is
// always false under `go test`, whatever a test does.
func Live() bool {
	if testing.Testing() {
		return false
	}
	return live.Load()
}

// otherCI lists environment variables that identify a CI system other than a
// dispatched GitHub Actions run. Their presence refuses paid commands.
var otherCI = []string{
	"BUILDKITE", "GITLAB_CI", "JENKINS_URL", "CIRCLECI", "TRAVIS", "TEAMCITY_VERSION",
	"BITBUCKET_BUILD_NUMBER", "TF_BUILD", "DRONE", "CODEBUILD_BUILD_ID", "APPVEYOR",
	"SEMAPHORE", "BUDDY", "WOODPECKER", "HARNESS_BUILD_ID", "CF_BUILD_ID",
}

// Request describes one attempt to start a paid command.
type Request struct {
	Command    string // run | resume
	MaxCostUSD float64
	// Yes is the workflow's --yes. It never replaces the typed confirmation
	// of a local run.
	Yes      bool
	StdinTTY bool
	Getenv   func(string) string
	// Preflight is printed before the local confirmation prompt.
	Preflight string
	In        io.Reader
	Out       io.Writer
}

// Decision is what Authorize concluded.
type Decision struct {
	CI      bool // a dispatched cli-evals.yml run
	Confirm bool // a typed confirmation is required
}

// Decide applies the policy without side effects or prompts.
func Decide(r Request) (Decision, error) {
	if err := CheckCap(r.MaxCostUSD); err != nil {
		return Decision{}, err
	}
	env := r.Getenv
	if env("GITHUB_ACTIONS") == "true" {
		if ev := env("GITHUB_EVENT_NAME"); ev != "workflow_dispatch" {
			return Decision{}, fmt.Errorf("refusing paid %q in GitHub Actions event %q: evals run only from a hand-dispatched %s (workflow_dispatch)", r.Command, ev, WorkflowFile)
		}
		if ref := env("GITHUB_WORKFLOW_REF"); !strings.Contains(ref, "/"+WorkflowFile+"@") {
			return Decision{}, fmt.Errorf("refusing paid %q from workflow %q: only %s may run paid evals", r.Command, ref, WorkflowFile)
		}
		if env("TWEVALS_APPROVED") != "1" {
			return Decision{}, errors.New("refusing paid run: TWEVALS_APPROVED=1 is set only by the job behind the evals environment gate")
		}
		return Decision{CI: true}, nil
	}
	if env("CI") != "" {
		return Decision{}, fmt.Errorf("refusing paid %q under CI=%q: the only CI path is a hand-dispatched %s", r.Command, env("CI"), WorkflowFile)
	}
	for _, k := range otherCI {
		if env(k) != "" {
			return Decision{}, fmt.Errorf("refusing paid %q: %s is set, and evals never run from %s", r.Command, k, k)
		}
	}
	if !r.StdinTTY {
		// A script or an agent's shell has no terminal: a local paid run
		// needs a person to type yes, and --yes does not stand in for one.
		return Decision{}, errors.New("stdin is not a terminal: a local paid run needs you to type yes on a terminal (--yes is for the dispatched workflow only)")
	}
	return Decision{Confirm: true}, nil
}

// CheckCap validates a --max-cost-usd value: a finite number above 0. NaN
// and Inf parse as floats and would otherwise lift every cap.
func CheckCap(usd float64) error {
	if math.IsNaN(usd) || math.IsInf(usd, 0) || usd <= 0 {
		return fmt.Errorf("--max-cost-usd is required for paid commands and must be a finite number greater than 0 (got %v; there is no default)", usd)
	}
	return nil
}

// Authorize applies the policy, prompts for the typed confirmation when one is
// needed, and unlocks Live() for the rest of the process.
func Authorize(r Request) error {
	d, err := Decide(r)
	if err != nil {
		return err
	}
	if d.Confirm {
		fmt.Fprintln(r.Out, r.Preflight)
		fmt.Fprintf(r.Out, "\nThis spends real money on OpenRouter (cap $%.2f). Type yes to continue: ", r.MaxCostUSD)
		line, _ := bufio.NewReader(r.In).ReadString('\n')
		if strings.TrimSpace(line) != "yes" {
			return errors.New("not confirmed; nothing was spent")
		}
	}
	if testing.Testing() {
		return nil
	}
	live.Store(true)
	return nil
}
