package grade

import (
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

func TestMCPAndCLIMapToTheSameOps(t *testing.T) {
	cli := ops.ParseCLI(strings.Fields("alerts snooze 12 --minutes 30 --yes"))
	mcp := ops.ParseMCP("mcp__traceway__snooze_notification_rule", map[string]any{"id": 12.0, "duration_minutes": 30.0})
	m := catalog.CallMatcher{Op: "alerts.snooze", Args: map[string]any{"id": "12", "duration_minutes": "30"}}
	for _, op := range []ops.Op{cli, mcp} {
		rec := &record.OpRecord{Op: op}
		if !Matches(rec, m) {
			t.Errorf("%+v does not match %+v", op, m)
		}
	}
	dis := ops.ParseMCP("set_notification_rule_enabled", map[string]any{"id": 3.0, "enabled": false})
	if dis.Op != "alerts.disable" || !dis.Write {
		t.Errorf("set_notification_rule_enabled(false) = %+v", dis)
	}
}

func TestDebugAnswer(t *testing.T) {
	debug := &Input{Case: &catalog.Case{Suite: "debug", Expect: catalog.Expect{Answer: &catalog.Answer{
		Category: "null_reference", Culprit: &catalog.Culprit{File: "internal/invoices/render.go", Symbol: "formatAddress", Line: 37}}}},
		Run: &record.Run{Submission: map[string]any{"category": "null_reference", "culprit": map[string]any{"file": "./internal/invoices/render.go", "symbol": "invoices.formatAddress", "line": 39.0}}}}
	if v := Answer(debug); !v.Pass {
		t.Fatalf("line within tolerance failed: %s", failures(v))
	}
	debug.Run.Submission["culprit"].(map[string]any)["line"] = 45.0
	if v := Answer(debug); v.Pass {
		t.Fatal("line 45 is outside 37±3")
	}
}
