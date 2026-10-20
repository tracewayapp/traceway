package seed

import (
	"testing"
	"time"
)

// TestHistogramDeltaWindows: each delta point starts where the previous one
// ended, across the chunks a series is rendered in, and counts the
// observations of exactly that window.
func TestHistogramDeltaWindows(t *testing.T) {
	m := &metricSeries{name: "h", kind: histogramKind, value: func(time.Time) float64 { return 5 }, count: func(time.Time) float64 { return 10 }}
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	var times []time.Time
	for i := range 6 {
		times = append(times, base.Add(time.Duration(i)*MetricStep.Old))
	}
	var points []struct {
		start, end uint64
		count      uint64
	}
	for _, chunk := range [][]time.Time{times[:3], times[3:]} {
		for _, dp := range m.render(chunk, base).GetHistogram().DataPoints {
			points = append(points, struct {
				start, end uint64
				count      uint64
			}{dp.StartTimeUnixNano, dp.TimeUnixNano, dp.Count})
		}
	}
	for i, p := range points {
		if p.count != 100 {
			t.Errorf("point %d counts %d observations, want 100 (10/min over 10 min)", i, p.count)
		}
		if i > 0 && p.start != points[i-1].end {
			t.Errorf("point %d starts at %d, want the previous point's end %d", i, p.start, points[i-1].end)
		}
		if got := time.Duration(p.end - p.start); got != MetricStep.Old {
			t.Errorf("point %d covers %s, want %s", i, got, MetricStep.Old)
		}
	}
}

// TestHistogramLowRateKeepsCount: a rate below one observation per point
// still reports its observations over time instead of rounding every point
// to zero, across the chunks a series is rendered in.
func TestHistogramLowRateKeepsCount(t *testing.T) {
	m := &metricSeries{name: "h", kind: histogramKind, value: func(time.Time) float64 { return 0.2 }, count: func(time.Time) float64 { return 0.3 }}
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	var times []time.Time
	for i := range 100 {
		times = append(times, base.Add(time.Duration(i)*time.Minute))
	}
	var total uint64
	for _, chunk := range [][]time.Time{times[:37], times[37:]} {
		for _, dp := range m.render(chunk, base).GetHistogram().DataPoints {
			total += dp.Count
		}
	}
	if total < 29 || total > 31 {
		t.Fatalf("100 minutes at 0.3/min reported %d observations, want 30", total)
	}

	m.histPrev, m.histCarry = time.Time{}, 0
	m.count = func(t time.Time) float64 {
		if t.Equal(base) {
			return 0.5
		}
		return 0
	}
	for _, dp := range m.render(times[:3], base).GetHistogram().DataPoints {
		if dp.Count > 1 {
			t.Fatalf("a rate that stops after half an observation reported %d at %d", dp.Count, dp.TimeUnixNano)
		}
	}
}

// TestRatioGaugeStaysInRange: the jitter never pushes a ratio past 1.
func TestRatioGaugeStaysInRange(t *testing.T) {
	s := NewScenario("ratio", time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC), NewDrySink())
	svc := s.Project("p", OTel).Service("svc", Go("nethttp", "sql"))
	svc.Gauge("cache.hit_ratio", "1", nil, Const(0.999))
	svc.Gauge("queue.depth", "{items}", nil, Const(1000))
	m, depth := svc.metrics[0], svc.metrics[1]
	over := false
	for i := range 600 {
		at := s.Now.Add(-time.Duration(i) * time.Minute)
		if v := m.value(at); v < 0 || v > 1 {
			t.Fatalf("ratio %v at %s", v, at)
		}
		over = over || depth.value(at) > 1000
	}
	if !over {
		t.Fatal("the jitter no longer moves other gauges above their series")
	}
}
