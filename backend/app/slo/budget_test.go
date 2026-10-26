package slo

import (
	"math"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func near(got, want float64) bool {
	return math.Abs(got-want) < 1e-6
}

func TestBudgetMath(t *testing.T) {
	target := 0.999
	window := Window{Good: 999_500, Total: 1_000_000}
	sli, ok := window.SLI()
	if !ok || !near(sli, 0.9995) {
		t.Fatalf("sli = %v, %v", sli, ok)
	}
	remaining, ok := BudgetRemaining(window, target)
	if !ok || !near(remaining, 50) {
		t.Fatalf("half the budget left: got %v", remaining)
	}
	overspent := Window{Good: 998_800, Total: 1_000_000}
	remaining, _ = BudgetRemaining(overspent, target)
	if !near(remaining, -20) {
		t.Fatalf("1200 bad of 1000 allowed is -20%%: got %v", remaining)
	}
	if burn := BurnRate(Window{Good: 9_856, Total: 10_000}, target); !near(burn, 14.4) {
		t.Fatalf("144 bad per 10000 at 99.9%% is 14.4x: got %v", burn)
	}
	if burn := BurnRate(Window{}, target); burn != 0 {
		t.Fatalf("no traffic burns nothing: got %v", burn)
	}
	if _, ok := BudgetRemaining(window, 1); ok {
		t.Fatal("a 100% target has no budget to measure")
	}
}

func TestStatus(t *testing.T) {
	if got := Status(Window{}, 0.99); got != models.SLOStatusNoData {
		t.Fatalf("empty window: %s", got)
	}
	if got := Status(Window{Good: 99, Total: 100}, 0.99); got != models.SLOStatusOk {
		t.Fatalf("exactly on target is ok: %s", got)
	}
	if got := Status(Window{Good: 98, Total: 100}, 0.99); got != models.SLOStatusBreached {
		t.Fatalf("below target is breached: %s", got)
	}
}

func TestSnapshot(t *testing.T) {
	now := time.Now().UTC()
	empty := Snapshot(Window{}, Window{}, Window{}, 0.99, now, nil, nil)
	if empty.Status != models.SLOStatusNoData || empty.CurrentSli != nil || empty.BudgetRemaining != nil || empty.BurnRate1h != nil {
		t.Fatalf("empty snapshot carries values: %+v", empty)
	}
	full := Snapshot(Window{Good: 995, Total: 1000}, Window{Good: 90, Total: 100}, Window{Good: 100, Total: 100}, 0.99, now, &now, &now)
	if full.Status != models.SLOStatusOk || !near(*full.CurrentSli, 99.5) || !near(*full.BudgetRemaining, 50) || !near(*full.BurnRate1h, 10) || !near(*full.BurnRate6h, 0) {
		t.Fatalf("snapshot: %+v", full)
	}
}
