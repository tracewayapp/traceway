//go:build telemetry_ch

package clickhouse

import "testing"

func TestIsSingleToken(t *testing.T) {
	cases := map[string]bool{
		"timeout":                 true,
		"Error42":                 true,
		"":                        false,
		"sync_deal_stage_to_keap": false,
		"two words":               false,
		"a.b":                     false,
		"naïve":                   false,
	}
	for in, want := range cases {
		if got := isSingleToken(in); got != want {
			t.Errorf("isSingleToken(%q) = %v, want %v", in, got, want)
		}
	}
}
