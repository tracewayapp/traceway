package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func perfPayload(locus any) map[string]any {
	return map[string]any{
		"checklist_item": "disk_io_bottleneck",
		"locus":          locus,
		"evidence":       []any{map[string]any{"call_id": "c01", "claim": "iowait"}},
		"fix":            "stop the rebuild job",
		"confidence":     "high",
		"report":         "Disk contention on pg-bookings-1.",
	}
}

func TestSubmitPerfRejectsMalformedLocus(t *testing.T) {
	good := perfPayload(map[string]any{"endpoint": "GET /api/availability", "service": "bookings-api", "host": "pg-bookings-1"})
	sub := &Submit{Kind: SubmitPerf, MaxRepairs: 2}
	if probs := sub.validate(good); len(probs) != 0 {
		t.Fatalf("a well-formed submission was rejected: %v", probs)
	}

	// The shape Opus 5.5 sent in 5 of 107 diagnoses: locus serialized as
	// tool-call markup, its fields flattened to the top level.
	bad := perfPayload("\n  <parameter name=\"service\">bookings-api")
	bad["endpoint"] = "GET /api/availability"
	bad["host"] = "pg-bookings-1"
	raw, _ := json.Marshal(bad)
	res := sub.Call(context.Background(), raw)
	if res.OK || res.Final {
		t.Fatalf("malformed locus was accepted: %+v", res)
	}
	for _, want := range []string{"locus must be an object", "locus contains tool-call markup"} {
		if !strings.Contains(res.Output, want) {
			t.Errorf("repair message lacks %q:\n%s", want, res.Output)
		}
	}
}

func TestSubmitRejectsMarkupInNestedFields(t *testing.T) {
	p := perfPayload(map[string]any{"endpoint": "GET /api/x"})
	p["evidence"] = []any{map[string]any{"call_id": "c01", "claim": `<parameter name="claim">x`}}
	probs := (&Submit{Kind: SubmitPerf}).validate(p)
	if len(probs) != 1 || !strings.HasPrefix(probs[0], "evidence[0].claim contains tool-call markup") {
		t.Fatalf("got %v", probs)
	}
}
