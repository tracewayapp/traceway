package shared

import (
	"math"
	"testing"
)

func TestNiceLogAttributeBins(t *testing.T) {
	cases := []struct {
		name               string
		min, max           float64
		target             int
		integral           bool
		start, width       float64
		count              int
		firstValue, lastIn float64
	}{
		{"round edges", 3.2, 97.1, 10, false, 0, 10, 10, 3.2, 97.1},
		{"negative values", -2.5, 100, 5, false, -25, 25, 6, -2.5, 100},
		{"whole numbers never get fractional bins", 1, 4, 40, true, 1, 1, 4, 1, 4},
		{"one value", 7, 7, 40, true, 7, 1, 1, 7, 7},
		{"one fractional value", 0.25, 0.25, 40, false, -0.25, 1, 1, 0.25, 0.25},
		{"small fractions", 0.001, 0.004, 3, false, 0.001, 0.001, 4, 0.001, 0.004},
	}
	for _, tc := range cases {
		b := NiceLogAttributeBins(tc.min, tc.max, tc.target, tc.integral)
		if math.Abs(b.Start-tc.start) > 1e-12 || math.Abs(b.Width-tc.width) > 1e-12 || b.Count != tc.count {
			t.Errorf("%s: got %+v, want start %v width %v count %d", tc.name, b, tc.start, tc.width, tc.count)
			continue
		}
		if b.Index(tc.firstValue) != 0 || b.Index(tc.lastIn) != b.Count-1 {
			t.Errorf("%s: min lands in bin %d and max in %d of %d", tc.name, b.Index(tc.firstValue), b.Index(tc.lastIn), b.Count)
		}
	}
}

func TestApplyLogAttributeQuantilesInterpolates(t *testing.T) {
	rows := []LogAttributeStatsRow{{Bucket: 60, Numeric: 4}, {Bucket: 120, Numeric: 1}, {Bucket: 180, Numeric: 0}}
	values := []LogAttributeQuantileRow{
		{Bucket: 60, N: 4, Position: 1, Value: 2},
		{Bucket: 60, N: 4, Position: 2, Value: 3},
		{Bucket: 60, N: 4, Position: 3, Value: 4},
		{Bucket: 120, N: 1, Position: 0, Value: 9},
	}
	ApplyLogAttributeQuantiles(rows, values)
	sorted := []float64{1, 2, 3, 4}
	for i, q := range LogAttributeQuantiles {
		got := []float64{rows[0].P50, rows[0].P95, rows[0].P99}[i]
		if want := ComputePercentile(sorted, q); math.Abs(got-want) > 1e-12 {
			t.Errorf("q%v = %v, want %v (the same interpolation as ComputePercentile)", q, got, want)
		}
	}
	if rows[1].P50 != 9 || rows[1].P99 != 9 {
		t.Errorf("a single value is every percentile, got %+v", rows[1])
	}
	if rows[2].P50 != 0 {
		t.Errorf("a bucket without numbers keeps zero percentiles, got %+v", rows[2])
	}
}
