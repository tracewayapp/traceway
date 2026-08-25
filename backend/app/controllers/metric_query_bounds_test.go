package controllers

import (
	"fmt"
	"testing"
	"time"
)

func bucketsFor(fromSec, rangeSec, intervalMinutes int) int {
	secs := intervalMinutes * 60
	return (fromSec+rangeSec)/secs - fromSec/secs + 1
}

func TestMinIntervalMinutesNeverExceedsTheCap(t *testing.T) {
	worst := 0
	for rangeMin := 1; rangeMin <= 20000; rangeMin++ {
		rangeSec := rangeMin * 60
		iv := minIntervalMinutes(rangeSec, metricQueryMaxBuckets)
		if iv < 1 {
			t.Fatalf("range %dm produced interval %d", rangeMin, iv)
		}
		for _, off := range []int{0, 1, 17, 31, 59, iv*60 - 1} {
			if n := bucketsFor(off, rangeSec, iv); n > worst {
				worst = n
			}
		}
	}
	if worst > metricQueryMaxBuckets {
		t.Fatalf("worst-case bucket count %d exceeds the %d cap", worst, metricQueryMaxBuckets)
	}
}

func TestValidateMetricQueryBounds(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	one := []MetricQueryItem{{Name: "cpu"}}

	manyFilters := map[string]string{}
	for i := 0; i <= metricQueryMaxTagFilters; i++ {
		manyFilters[fmt.Sprintf("k%d", i)] = "v"
	}
	manyQueries := make([]MetricQueryItem, metricQueryMaxQueries+1)
	for i := range manyQueries {
		manyQueries[i] = MetricQueryItem{Name: "cpu"}
	}

	cases := []struct {
		name    string
		req     MetricQueryRequest
		rejects bool
	}{
		{"ordinary range", MetricQueryRequest{Queries: one, From: base, To: base.Add(time.Hour)}, false},
		{"at the range cap", MetricQueryRequest{Queries: one, From: base, To: base.Add(metricQueryMaxRange)}, false},
		{"reversed", MetricQueryRequest{Queries: one, From: base.Add(time.Hour), To: base}, true},
		{"equal", MetricQueryRequest{Queries: one, From: base, To: base}, true},
		{"over the range cap", MetricQueryRequest{Queries: one, From: base, To: base.Add(metricQueryMaxRange + time.Hour)}, true},
		{"absurd timestamps", MetricQueryRequest{Queries: one, From: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)}, true},
		{"too many tag filters", MetricQueryRequest{Queries: []MetricQueryItem{{Name: "cpu", TagFilters: manyFilters}}, From: base, To: base.Add(time.Hour)}, true},
		{"too many queries", MetricQueryRequest{Queries: manyQueries, From: base, To: base.Add(time.Hour)}, true},
		{"interval over the ceiling", MetricQueryRequest{Queries: one, From: base, To: base.Add(time.Hour), IntervalMinutes: metricQueryMaxIntervalMinutes + 1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.req
			msg := validateMetricQuery(&req)
			if tc.rejects && msg == "" {
				t.Fatal("expected a validation message")
			}
			if !tc.rejects && msg != "" {
				t.Fatalf("unexpected rejection: %s", msg)
			}
			if !tc.rejects && req.IntervalMinutes < 1 {
				t.Fatalf("interval was not defaulted: %d", req.IntervalMinutes)
			}
		})
	}
}
