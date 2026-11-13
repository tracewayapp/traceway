package shared

import (
	"testing"
	"time"
)

func TestLogHistogramStepPicksRoundStepWithinBudget(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		span time.Duration
		want int64
	}{
		{30 * time.Minute, 30},
		{time.Hour, 60},
		{24 * time.Hour, 1800},
		{7 * 24 * time.Hour, 10800},
		{90 * 24 * time.Hour, 172800},
	}
	for _, tc := range cases {
		got := LogHistogramStep(base, base.Add(tc.span), LogHistogramDefaultBuckets)
		if got != tc.want {
			t.Errorf("span %s: step %d, want %d", tc.span, got, tc.want)
		}
		if n := logHistogramBucketCount(base, base.Add(tc.span), got); n > LogHistogramDefaultBuckets {
			t.Errorf("span %s: %d buckets over budget", tc.span, n)
		}
	}
}

func TestLogHistogramStepFallsBackToWholeDays(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(2 * 365 * 24 * time.Hour)
	step := LogHistogramStep(from, to, 60)
	if step%86400 != 0 {
		t.Fatalf("step %d is not whole days", step)
	}
	if n := logHistogramBucketCount(from, to, step); n > 60 {
		t.Fatalf("%d buckets over budget", n)
	}
}

func TestBuildLogHistogramFillsGapsAndPlacesRows(t *testing.T) {
	from := time.Date(2026, 3, 1, 10, 0, 30, 0, time.UTC)
	to := time.Date(2026, 3, 1, 10, 4, 59, 999_000_000, time.UTC)
	first := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC).Unix()
	rows := []LogHistogramRow{
		{Bucket: first, Level: LogLevelError, Count: 2},
		{Bucket: first, Level: LogLevelInfo, Count: 5},
		{Bucket: first + 180, Level: LogLevelWarn, Count: 1},
		{Bucket: first + 180, Level: 42, Count: 3},
		{Bucket: first + 600, Level: LogLevelInfo, Count: 9},
		{Bucket: first - 60, Level: LogLevelInfo, Count: 9},
	}
	buckets := BuildLogHistogram(from, to, 60, rows)
	if len(buckets) != 5 {
		t.Fatalf("got %d buckets, want 5", len(buckets))
	}
	if !buckets[0].Start.Equal(time.Unix(first, 0)) {
		t.Errorf("first bucket starts %s", buckets[0].Start)
	}
	if buckets[0].Error != 2 || buckets[0].Info != 5 || buckets[0].Total() != 7 {
		t.Errorf("bucket 0 = %+v", buckets[0])
	}
	if buckets[3].Warn != 1 || buckets[3].Unknown != 3 {
		t.Errorf("bucket 3 = %+v", buckets[3])
	}
	var total int64
	for _, b := range buckets {
		total += b.Total()
	}
	if total != 11 {
		t.Errorf("total %d, want 11: rows outside the range must be dropped", total)
	}
}

func TestBuildLogHistogramReversedRangeIsEmpty(t *testing.T) {
	now := time.Now()
	if got := BuildLogHistogram(now, now.Add(-time.Hour), 60, nil); len(got) != 0 {
		t.Fatalf("got %d buckets for a reversed range", len(got))
	}
}
