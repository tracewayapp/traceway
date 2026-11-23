package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

// seedChartSpans stores five spans in two ten-minute buckets, one of them twice
// (a retried export whose shorter version must lose), plus a span of another
// project. It returns the search that matches the five.
func seedChartSpans(t *testing.T) (shared.OtelSpanSearch, time.Time) {
	t.Helper()
	setupTestDB(t)
	ctx := context.Background()
	project := uuid.New()
	base := time.Now().UTC().Truncate(time.Hour).Add(-2 * time.Hour)
	build := func(owner uuid.UUID, name string, duration time.Duration, minute int, attributes map[string]string) models.OtelSpan {
		span := canonicalSpan(owner, uuid.New(), uuid.New(), nil)
		start := base.Add(time.Duration(minute) * time.Minute)
		span.Name, span.ServiceName, span.Duration, span.StartTime, span.RecordedAt, span.Attributes = name, "checkout-api", duration, start, start, attributes
		return span
	}
	slowMiss := build(project, "GET product", 60*time.Millisecond, 11, map[string]string{"cache.hit": "false", "db.rows": "30"})
	retried := slowMiss
	retried.Duration = 50 * time.Millisecond
	spans := []models.OtelSpan{
		build(project, "GET product", 2*time.Millisecond, 1, map[string]string{"cache.hit": "true", "db.rows": "5"}),
		build(project, "GET product", 4*time.Millisecond, 1, map[string]string{"cache.hit": "true", "db.rows": "abc"}),
		build(project, "GET product", 40*time.Millisecond, 11, map[string]string{"cache.hit": "false", "db.rows": "20"}),
		slowMiss,
		build(project, "POST checkout", time.Second, 11, nil),
		build(uuid.New(), "GET product", 7*time.Second, 11, map[string]string{"cache.hit": "false", "db.rows": "999"}),
	}
	if _, err := OtelSpanRepository.InsertAsync(ctx, spans); err != nil {
		t.Fatal(err)
	}
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{retried}); err != nil {
		t.Fatal(err)
	}
	return shared.OtelSpanSearch{ProjectId: project, From: base, To: base.Add(30 * time.Minute), Page: 1, PageSize: 50}, base
}

func TestOtelSpanSearchAttributeOperators(t *testing.T) {
	search, _ := seedChartSpans(t)
	ctx := context.Background()
	for _, tt := range []struct {
		name   string
		filter shared.OtelAttributeFilter
		total  uint64
	}{
		{"equals", shared.OtelAttributeFilter{Key: "cache.hit", Value: "false"}, 2},
		{"not equals keeps spans without the key", shared.OtelAttributeFilter{Key: "cache.hit", Value: "true", Exclude: true}, 3},
		{"contains, any case", shared.OtelAttributeFilter{Key: "db.rows", Value: "AB", Contains: true}, 1},
		{"not contains keeps spans without the key", shared.OtelAttributeFilter{Key: "db.rows", Value: "ab", Contains: true, Exclude: true}, 4},
		{"key exists", shared.OtelAttributeFilter{Key: "cache.hit", Exists: true}, 4},
		{"key does not exist", shared.OtelAttributeFilter{Key: "cache.hit", Exists: true, Exclude: true}, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			filtered := search
			filtered.Attributes = []shared.OtelAttributeFilter{tt.filter}
			_, total, err := OtelSpanRepository.Search(ctx, filtered)
			if err != nil || total != tt.total {
				t.Fatalf("got %d spans (%v), want %d", total, err, tt.total)
			}
		})
	}
}

func TestOtelSpanChartDurationPerBucket(t *testing.T) {
	search, base := seedChartSpans(t)
	rows, err := OtelSpanRepository.ChartStats(context.Background(), search, "", 600)
	if err != nil {
		t.Fatal(err)
	}
	buckets, present, numeric := shared.BuildChartSeries(rows)
	if present != 5 || numeric != 5 || len(buckets) != 2 {
		t.Fatalf("got %d spans, %d durations in %d buckets, want 5 in 2 (the retried span once): %+v", present, numeric, len(buckets), buckets)
	}
	first, second := buckets[0], buckets[1]
	if !first.Start.Equal(base) || first.Count != 2 || !near(first.Avg, 3) || !near(first.Min, 2) || !near(first.Max, 4) {
		t.Errorf("first bucket = %+v, want 2 spans of 2 and 4 ms starting %v", first, base)
	}
	if !second.Start.Equal(base.Add(10*time.Minute)) || second.Count != 3 || !near(second.Sum, 1100) || !near(second.Max, 1000) || !near(second.P50, 60) {
		t.Errorf("second bucket = %+v, want 40, 60 (not the retry's 50) and 1000 ms", second)
	}
}

func TestOtelSpanChartDurationDistribution(t *testing.T) {
	search, _ := seedChartSpans(t)
	ctx := context.Background()
	stats, err := OtelSpanRepository.ChartStats(ctx, search, "", 0)
	if err != nil || len(stats) != 1 {
		t.Fatalf("whole range = %+v %v", stats, err)
	}
	row := stats[0]
	if row.Numeric != 5 || !near(row.Min, 2) || !near(row.Max, 1000) || !near(row.Sum, 1106) || !near(row.P50, 40) {
		t.Fatalf("whole range = %+v, want 5 durations from 2 to 1000 ms with median 40", row)
	}
	binning := shared.NiceLogScaleBins(row.Min, row.Max, shared.ChartDefaultBins)
	binRows, err := OtelSpanRepository.ChartBins(ctx, search, "", binning)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]int64{}
	for _, r := range binRows {
		got[r.Index] += r.Count
	}
	want := map[int]int64{}
	for _, ms := range []float64{2, 4, 40, 60, 1000} {
		want[binning.Index(ms)]++
	}
	if len(got) != len(want) {
		t.Fatalf("log-scale bins = %v, want %v (the same bins Go computes)", got, want)
	}
	for index, count := range want {
		if got[index] != count {
			t.Fatalf("log-scale bins = %v, want %v (the same bins Go computes)", got, want)
		}
	}
}

func TestOtelSpanChartAttribute(t *testing.T) {
	search, _ := seedChartSpans(t)
	ctx := context.Background()
	stats, err := OtelSpanRepository.ChartStats(ctx, search, "db.rows", 0)
	if err != nil || len(stats) != 1 {
		t.Fatalf("whole range = %+v %v", stats, err)
	}
	row := stats[0]
	if row.Present != 4 || row.Numeric != 3 || row.Integral != 3 || !near(row.Sum, 55) || !near(row.P50, 20) {
		t.Fatalf("db.rows = %+v, want 4 spans carrying it, 3 numbers summing to 55", row)
	}
	binning := shared.NiceChartBins(row.Min, row.Max, shared.ChartDefaultBins, true)
	binRows, err := OtelSpanRepository.ChartBins(ctx, search, "db.rows", binning)
	if err != nil {
		t.Fatal(err)
	}
	bins := shared.BuildChartBins(binning, binRows)
	var counted int64
	for _, bin := range bins {
		counted += bin.Count
	}
	if counted != 3 || bins[0].Count != 1 || bins[len(bins)-1].Count != 1 {
		t.Fatalf("db.rows bins = %+v, want 5 and 30 at the ends", bins)
	}

	filtered := search
	filtered.Attributes = []shared.OtelAttributeFilter{{Key: "cache.hit", Value: "false"}}
	stats, err = OtelSpanRepository.ChartStats(ctx, filtered, "db.rows", 0)
	if err != nil || len(stats) != 1 || stats[0].Numeric != 2 || !near(stats[0].Max, 30) {
		t.Fatalf("the chart follows the search's filters: %+v %v", stats, err)
	}
	stats, err = OtelSpanRepository.ChartStats(ctx, search, "no.such.key", 0)
	if err != nil || len(stats) != 0 {
		t.Fatalf("an attribute no span carries charts nothing: %+v %v", stats, err)
	}
}

// A span stored twice counts as its winning version (the longer one) everywhere: an attribute only the losing
// version carries is not charted, as the search does not find the span by it either.
func TestOtelSpanChartAttributeOfTheWinningVersion(t *testing.T) {
	search, base := seedChartSpans(t)
	ctx := context.Background()
	winner := canonicalSpan(search.ProjectId, uuid.New(), uuid.New(), nil)
	start := base.Add(21 * time.Minute)
	winner.Name, winner.ServiceName, winner.Duration, winner.StartTime, winner.RecordedAt = "GET product", "checkout-api", 90*time.Millisecond, start, start
	winner.Attributes = map[string]string{"cache.hit": "false"}
	loser := winner
	loser.Duration = 80 * time.Millisecond
	loser.Attributes = map[string]string{"cache.hit": "false", "db.rows": "77"}
	for _, span := range []models.OtelSpan{winner, loser} {
		if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{span}); err != nil {
			t.Fatal(err)
		}
	}
	for name, chartSearch := range map[string]shared.OtelSpanSearch{
		"no filter": search,
		"filtered": func() shared.OtelSpanSearch {
			filtered := search
			filtered.Attributes = []shared.OtelAttributeFilter{{Key: "cache.hit", Value: "false"}}
			return filtered
		}(),
	} {
		stats, err := OtelSpanRepository.ChartStats(ctx, chartSearch, "db.rows", 0)
		if err != nil || len(stats) != 1 {
			t.Fatalf("%s: whole range = %+v %v", name, stats, err)
		}
		if near(stats[0].Max, 77) || !near(stats[0].Max, 30) {
			t.Errorf("%s: db.rows = %+v, want a maximum of 30: the value 77 is on the version that lost", name, stats[0])
		}
		withKey := chartSearch
		withKey.Attributes = append(append([]shared.OtelAttributeFilter{}, chartSearch.Attributes...), shared.OtelAttributeFilter{Key: "db.rows", Exists: true})
		_, total, err := OtelSpanRepository.Search(ctx, withKey)
		if err != nil || int64(total) != stats[0].Present {
			t.Errorf("%s: the chart reads %d spans, the search with the key finds %d (%v)", name, stats[0].Present, total, err)
		}
	}
}
