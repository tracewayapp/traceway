package controllers

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestValidateSLIDefinitionRequiresAMetricThreshold(t *testing.T) {
	base := map[string]any{"metricName": "cpu.used_pcnt", "aggregation": "avg", "operator": "lt"}
	for name, threshold := range map[string]any{"missing": nil, "zero": 0.0, "set": 90.0} {
		cfg := map[string]any{}
		for k, v := range base {
			cfg[k] = v
		}
		if name != "missing" {
			cfg["threshold"] = threshold
		}
		raw, _ := json.Marshal(cfg)
		_, message, _ := validateSLIDefinition(nil, uuid.New(), "metric", raw, 5)
		if name == "missing" && message != "Threshold is required." {
			t.Fatalf("missing threshold: got %q", message)
		}
		if name != "missing" && message != "" {
			t.Fatalf("%s threshold: got %q", name, message)
		}
	}

	raw := json.RawMessage(`{"metricName":"cpu.used_pcnt","aggregation":"avg","operator":"lt","threshold":null}`)
	if _, message, _ := validateSLIDefinition(nil, uuid.New(), "metric", raw, 5); message != "Threshold is required." {
		t.Fatalf("null threshold: got %q", message)
	}
}

func TestSameRuleConfigIgnoresKeyOrder(t *testing.T) {
	a := json.RawMessage(`{"sloId":1,"longWindowMinutes":60,"shortWindowMinutes":5,"burnRateThreshold":14.4}`)
	b := json.RawMessage(`{"burnRateThreshold":14.4,"shortWindowMinutes":5,"longWindowMinutes":60,"sloId":1}`)
	if !sameRuleConfig(a, b) {
		t.Fatal("reordered keys should compare equal")
	}
	c := json.RawMessage(`{"sloId":1,"longWindowMinutes":60,"shortWindowMinutes":5,"burnRateThreshold":6}`)
	if sameRuleConfig(a, c) {
		t.Fatal("a changed threshold should compare different")
	}
}

func TestValidateSLIDefinitionSpanAttributesMatchTheExplorer(t *testing.T) {
	cases := map[string]string{
		`{"attributes":[{"key":"tenant","value":""}],"good":"availability"}`:     "An attribute filter needs a value.",
		`{"attributes":[{"key":"te\"nant","value":"a"}],"good":"availability"}`:  "Attribute keys cannot contain quotes or backslashes.",
		`{"attributes":[{"key":"tenant","value":"acme"}],"good":"availability"}`: "",
	}
	for raw, want := range cases {
		if _, got, _ := validateSLIDefinition(nil, uuid.New(), "span", json.RawMessage(raw), 1); got != want {
			t.Fatalf("%s: got %q, want %q", raw, got, want)
		}
	}
}

func TestValidateSLIDefinitionCapsMonitors(t *testing.T) {
	ids := make([]int, 51)
	for i := range ids {
		ids[i] = i + 1
	}
	raw, _ := json.Marshal(map[string]any{"checkIds": ids})
	if _, got, _ := validateSLIDefinition(nil, uuid.New(), "monitor", raw, 1); got != "At most 50 monitors are allowed." {
		t.Fatalf("got %q", got)
	}
}
