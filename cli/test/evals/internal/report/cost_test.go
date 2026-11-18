package report

import (
	"math"
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
)

func TestRunCost(t *testing.T) {
	opus := LivePrice{Price: catalog.Price{Prompt: 4, Completion: 20, CacheRead: 0.2, CacheWrite: 5}}
	// 100k input, 70% cached, 5k output: 30k at the write price, 70k at the read price, 5k out.
	got := RunCost(opus, "anthropic", 100000, 5000, 0.7)
	want := (30000*5.0 + 70000*0.2 + 5000*20.0) / 1e6
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("RunCost = %v, want %v", got, want)
	}
}
