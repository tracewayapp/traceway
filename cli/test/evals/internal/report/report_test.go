package report

import (
	"math"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/grade"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/runner"
)

func res(c, arm, model string, idx int, status string, cost float64) *runner.Result {
	suite := c[:len("perf")]
	if c[:5] == "debug" {
		suite = "debug"
	}
	return &runner.Result{Case: c, Suite: suite, Arm: arm, Model: model, RunIdx: idx, Status: status, Metrics: runner.Metrics{CostUSD: cost, ToolCalls: 10}}
}

func TestAggregate(t *testing.T) {
	runs := []*runner.Result{
		res("perf-a", "cli+skill", "opus", 1, "pass", 0.5),
		res("perf-a", "cli-noskill", "opus", 1, "fail", 0.4),
		res("perf-b", "cli+skill", "opus", 1, "pass", 0.7),
		res("perf-b", "cli-noskill", "opus", 1, "pass", 0.6),
		res("perf-c", "cli+skill", "opus", 1, "infra_error", 0),
	}
	a := Aggregate(runs)
	if r := a.Suites["perf|opus|cli+skill"]; r.Pass != 2 || r.Total != 2 {
		t.Fatalf("infra errors must not count: %+v", r)
	}
	if len(a.SkillUplift) != 1 || math.Abs(a.SkillUplift[0].Delta-50) > 1e-9 {
		t.Fatalf("uplift %+v", a.SkillUplift)
	}
	if c := a.Cost["opus"]; c.PassingRuns != 3 || math.Abs(c.AvgCostUSD-0.6) > 1e-9 {
		t.Fatalf("cost %+v", c)
	}
}

// TestEfficiency: going over a thrash limit is counted beside the pass rate,
// not in it.
func TestEfficiency(t *testing.T) {
	over := res("perf-a", "cli+skill", "opus", 1, "pass", 0.5)
	over.Metrics.FailedCalls = 4
	over.Graders = []*grade.Verdict{{Grader: "answer", Pass: true, Gating: true}, {Grader: "efficiency", Pass: false, Failures: []string{"the same call ran 3 times"}}}
	runs := []*runner.Result{over, res("perf-b", "cli+skill", "opus", 1, "pass", 0.5)}
	a := Aggregate(runs)
	if r := a.Suites["perf|opus|cli+skill"]; r.Pass != 2 {
		t.Fatalf("an efficiency miss changed the pass rate: %+v", r)
	}
	e := a.Efficiency["opus|cli+skill"]
	if e == nil || e.Runs != 2 || e.OverLimit != 1 || e.AvgFailedCalls != 2 || len(a.TopFailing) != 0 {
		t.Fatalf("efficiency %+v, top failing %+v", e, a.TopFailing)
	}
	out := Markdown(&Results{Runs: runs, Aggregates: a})
	if !strings.Contains(out, "| opus | cli+skill | 2 | 10.0 | 2.0 | 1 |") {
		t.Errorf("summary lacks the efficiency row:\n%s", out)
	}
	if strings.Contains(out, "efficiency:") {
		t.Errorf("an efficiency miss was reported as a run's failure:\n%s", out)
	}
}

// TestOracleSummary: an oracle run costs nothing, so its summary names no
// spend, and each run's transcript is listed at the path it was written to.
func TestOracleSummary(t *testing.T) {
	r := res("perf-a", "cli+skill", "oracle", 1, "pass", 0)
	r.Transcript = "runs/perf-a/cli_skill/oracle/r1/transcript.jsonl"
	out := Markdown(&Results{Header: Header{Oracle: true}, Runs: []*runner.Result{r}, Aggregates: Aggregate([]*runner.Result{r})})
	for _, s := range []string{"Spend", "Cost", "cap"} {
		if strings.Contains(out, s) {
			t.Errorf("oracle summary mentions %q:\n%s", s, out)
		}
	}
	if !strings.Contains(out, r.Transcript) {
		t.Errorf("summary lacks the transcript path:\n%s", out)
	}
	paid := Markdown(&Results{Runs: []*runner.Result{r}, Aggregates: Aggregate([]*runner.Result{r})})
	if !strings.Contains(paid, "Spend") || !strings.Contains(paid, "| Cost |") {
		t.Errorf("a paid run's summary lost its cost:\n%s", paid)
	}
}
