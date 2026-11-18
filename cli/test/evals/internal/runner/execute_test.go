package runner

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/agent"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
)

// fakeRuns replaces runOne: each run spends spend(i) through its hold and
// records the cap it was given.
func fakeRuns(t *testing.T, spend func(n int) float64) (caps *[]float64) {
	t.Helper()
	var mu sync.Mutex
	var got []float64
	n := 0
	orig := startRun
	startRun = func(_ context.Context, _ Options, j Job, hold *agent.Hold) *Result {
		mu.Lock()
		got = append(got, hold.Cap)
		i := n
		n++
		mu.Unlock()
		hold.Add("m", min(spend(i), hold.Cap))
		return &Result{Case: j.Unit.Case.ID, Arm: j.Unit.Arm, Model: j.Model.Key, RunIdx: j.Idx, Status: "pass"}
	}
	t.Cleanup(func() { startRun = orig })
	return &got
}

func jobsN(n int) []Job {
	m := &catalog.Model{Key: "m"}
	var out []Job
	for i := 1; i <= n; i++ {
		out = append(out, Job{Unit: catalog.Unit{Case: &catalog.Case{ID: fmt.Sprintf("perf-c%d", i), Suite: "perf"}, Arm: "cli+skill"}, Model: m, Idx: 1})
	}
	return out
}

// TestExecuteSharesTheCapAmongTheRunsLeft: two runs at -j 4 each get half the
// shard's cap, not a quarter.
func TestExecuteSharesTheCapAmongTheRunsLeft(t *testing.T) {
	caps := fakeRuns(t, func(int) float64 { return 0.5 })
	ledger := agent.NewLedger(3.75)
	res := Execute(context.Background(), Options{Jobs: 4, Ledger: ledger}, jobsN(2))
	if len(res) != 2 {
		t.Fatalf("%d results", len(res))
	}
	for _, c := range *caps {
		if math.Abs(c-1.875) > 1e-9 {
			t.Fatalf("caps %v, want 1.875 each", *caps)
		}
	}
}

// TestExecuteNeverPromisesMoreThanTheCap: with every run spending its whole
// reservation, the total stays at the cap and the runs that no longer fit
// are recorded as cost_ceiling, not started.
func TestExecuteNeverPromisesMoreThanTheCap(t *testing.T) {
	fakeRuns(t, func(int) float64 { return 100 })
	ledger := agent.NewLedger(5)
	res := Execute(context.Background(), Options{Jobs: 4, Ledger: ledger}, jobsN(12))
	if ledger.Spent() > 5+1e-9 {
		t.Fatalf("spent $%.2f of a $5 cap", ledger.Spent())
	}
	ceiling := 0
	for _, r := range res {
		if r.Status == agent.StopCeiling {
			ceiling++
		}
	}
	if len(res) != 12 || ceiling == 0 {
		t.Fatalf("%d results, %d at the ceiling", len(res), ceiling)
	}
}

// TestExecuteResumeSharesAmongTheRemainingRuns: skipped (already finished)
// runs do not shrink the share of the ones a resume still has to do.
func TestExecuteResumeSharesAmongTheRemainingRuns(t *testing.T) {
	caps := fakeRuns(t, func(int) float64 { return 0.1 })
	jobs := jobsN(60)
	skip := map[string]bool{}
	for _, j := range jobs[2:] {
		skip[RunKey(j.Unit.Case.ID, j.Unit.Arm, j.Model.Key, j.Idx)] = true
	}
	res := Execute(context.Background(), Options{Jobs: 4, Ledger: agent.NewLedger(5), Skip: skip}, jobs)
	if len(res) != 2 || len(*caps) != 2 || (*caps)[0] < 2.5-1e-9 {
		t.Fatalf("%d results, caps %v: each of the 2 remaining runs should get at least half of $5", len(res), *caps)
	}
}
