package notifications

import (
	"math"
	"testing"
)

func TestLatencyBreached(t *testing.T) {
	cases := []struct {
		name      string
		value     float64
		threshold float64
		want      bool
	}{
		{"empty window", math.NaN(), 10000, false},
		{"below", 4709, 10000, false},
		{"at threshold", 10000, 10000, true},
		{"above", 12000, 10000, true},
	}
	for _, c := range cases {
		if got := latencyBreached(c.value, c.threshold); got != c.want {
			t.Errorf("%s: latencyBreached(%v, %v) = %v, want %v", c.name, c.value, c.threshold, got, c.want)
		}
	}
}
