// Package grade scores one run. Deterministic graders gate the result:
// answer, trajectory, grounding, state and hidden tests. The LLM judge scores
// report quality and does not gate until it is calibrated against human
// labels.
package grade

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/agent"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

// Verdict is one grader's result.
type Verdict struct {
	Grader   string   `json:"grader"`
	Pass     bool     `json:"pass"`
	Gating   bool     `json:"gating"`
	Skipped  bool     `json:"skipped,omitempty"`
	Score    *float64 `json:"score,omitempty"`
	Failures []string `json:"failures,omitempty"`
	Notes    []string `json:"notes,omitempty"`
	CostUSD  float64  `json:"costUsd,omitempty"`
}

func (v *Verdict) fail(format string, a ...any) {
	v.Pass = false
	v.Failures = append(v.Failures, fmt.Sprintf(format, a...))
}

func (v *Verdict) note(format string, a ...any) { v.Notes = append(v.Notes, fmt.Sprintf(format, a...)) }

func newVerdict(name string, gating bool) *Verdict {
	return &Verdict{Grader: name, Pass: true, Gating: gating}
}

// StateReader reads backend state for the state grader. The runner
// implements it over the instance's admin API.
type StateReader interface {
	Get(ctx context.Context, path string, query map[string]string) (int, []byte, error)
	Post(ctx context.Context, path string, query map[string]string, body any) (int, []byte, error)
	ProjectID(name string) string
	PrimaryProject() string
	OrgID() int
	Tokens() []string
}

// Input is everything the graders see.
type Input struct {
	Case      *catalog.Case // templates resolved
	Arm       string
	Run       *record.Run
	Seed      map[string]any
	Workspace string
	State     StateReader
	// FakeModel is set for oracle runs: hidden tests may run on the host,
	// and the judge is skipped.
	FakeModel bool
	// AllowHostTests lets hidden tests of a real model's patch run on the
	// host when no container runtime is available.
	AllowHostTests bool
	// SandboxImage runs hidden tests in a container.
	SandboxImage string
	EvalsRoot    string
	Judge        *Judge
	// Pristine is the workspace's Snapshot before the agent started; the
	// hidden-tests grader diffs against it.
	Pristine map[string]string
	// Hold is the run's reservation of the cost cap; the judge's spending
	// goes there (nil: straight to the judge's ledger).
	Hold *agent.Hold
	// VerifyStart is when the setup verify step started. endpoint_seen
	// counts only telemetry from then on, so the agent's own test runs
	// cannot stand in for the production start command.
	VerifyStart time.Time
}

// All runs every applicable grader in a fixed order.
func All(ctx context.Context, in *Input) []*Verdict {
	var out []*Verdict
	if in.Case.Expect.Answer != nil {
		out = append(out, Answer(in))
	}
	if in.Case.Expect.Calls != nil {
		out = append(out, Trajectory(in.Run, in.Case.Expect.Calls))
	}
	if in.Case.Suite == "perf" || in.Case.Suite == "debug" || len(in.Case.Expect.Grounding) > 0 {
		out = append(out, Grounding(in))
	}
	if len(in.Case.Expect.State) > 0 {
		out = append(out, State(ctx, in))
	}
	if in.Case.Expect.Code != nil {
		out = append(out, HiddenTests(ctx, in))
	}
	if in.Case.Expect.Report != nil {
		out = append(out, Report(in))
	}
	if len(in.Case.Judge.Rubric) > 0 {
		out = append(out, in.Judge.Grade(ctx, in))
	}
	return out
}

// Passed reports whether every gating grader passed.
func Passed(vs []*Verdict) bool {
	for _, v := range vs {
		if v.Gating && !v.Skipped && !v.Pass {
			return false
		}
	}
	return true
}

// FinalText is the agent's report: the submission's report/summary field,
// else its last message.
func FinalText(r *record.Run) string {
	if r.Submission != nil {
		for _, k := range []string{"report", "summary"} {
			if s, ok := r.Submission[k].(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return r.FinalText
}
