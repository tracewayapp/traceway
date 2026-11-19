package report

import (
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/runner"
)

func TestCompare(t *testing.T) {
	base := NewBaseline(&Results{Runs: []*runner.Result{
		res("perf-a", "cli+skill", "opus", 1, "pass", 0), res("perf-a", "cli+skill", "opus", 2, "pass", 0), res("perf-a", "cli+skill", "opus", 3, "fail", 0),
		res("perf-b", "cli+skill", "opus", 1, "pass", 0),
	}})
	now := &Results{Runs: []*runner.Result{res("perf-a", "cli+skill", "opus", 1, "fail", 0), res("perf-b", "cli+skill", "opus", 1, "pass", 0)}}
	regs := Compare(now, base)
	kinds := map[string]bool{}
	for _, r := range regs {
		kinds[r.Kind] = true
	}
	if !kinds["suite"] || !kinds["case"] {
		t.Fatalf("want a suite drop and a case regression, got %+v", regs)
	}
	if regs := Compare(&Results{Runs: base.casesAsRuns()}, base); len(regs) != 0 {
		t.Fatalf("no change must not regress: %+v", regs)
	}
	once := NewBaseline(&Results{Runs: []*runner.Result{res("perf-b", "cli+skill", "opus", 1, "pass", 0), res("perf-c", "cli+skill", "opus", 1, "pass", 0)}})
	for _, r := range Compare(&Results{Runs: []*runner.Result{res("perf-b", "cli+skill", "opus", 1, "fail", 0), res("perf-c", "cli+skill", "opus", 1, "pass", 0)}}, once) {
		if r.Kind == "case" {
			t.Fatalf("a case that passed once in the baseline is not a case regression after one failure: %+v", r)
		}
	}
}

// casesAsRuns rebuilds runs matching the baseline exactly.
func (b *Baseline) casesAsRuns() []*runner.Result {
	var out []*runner.Result
	for k, r := range b.Cases {
		c, m, arm := split3(k)
		for i := 0; i < r.Total; i++ {
			st := "fail"
			if i < r.Pass {
				st = "pass"
			}
			out = append(out, res(c, arm, m, i+1, st, 0))
		}
	}
	return out
}
