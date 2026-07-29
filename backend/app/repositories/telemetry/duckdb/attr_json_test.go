//go:build telemetry_duckdb

package duckdb

import (
	"encoding/json"
	"testing"
)

// attrJSON output must stay byte-identical to encoding/json: existing rows
// were written with stdlib and read paths compare/extract against that shape.
func TestAttrJSONMatchesStdlib(t *testing.T) {
	cases := []map[string]string{
		nil,
		{},
		{"key": "value"},
		{"b": "2", "a": "1", "c": "3"},
		{"html": "<script>&'\"</script>"},
		{"escape": "line1\nline2\ttab \\ backslash"},
		{"unicode": "héllo wörld 日本語 🚀"},
		{"invalid-utf8": "bad\xff\xfebytes"},
		{"empty-value": "", "": "empty-key"},
		{"control": "\x00\x1f"},
	}

	for _, m := range cases {
		got, err := attrJSON(m)
		if err != nil {
			t.Fatalf("attrJSON(%v) failed: %v", m, err)
		}
		if len(m) == 0 {
			if got != "{}" {
				t.Errorf("attrJSON(%v) = %q, want {}", m, got)
			}
			continue
		}
		want, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("json.Marshal(%v) failed: %v", m, err)
		}
		if got != string(want) {
			t.Errorf("attrJSON(%v) = %q, stdlib = %q", m, got, want)
		}
	}
}
