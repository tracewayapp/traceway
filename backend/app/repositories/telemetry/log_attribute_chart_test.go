package telemetry

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

type chartFixture struct {
	ctx       context.Context
	projectId uuid.UUID
	base      time.Time
	params    shared.LogSearchParams
}

// seedChartLogs stores the same records on every backend: three numbers in the
// first minute, three in the second (one negative, one in exponent form), four
// values that are not JSON numbers in the third, and logs without the attribute.
func seedChartLogs(t *testing.T) chartFixture {
	t.Helper()
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	// Recent on purpose: ClickHouse's log_records TTL drops rows older than 90 days.
	base := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)

	record := func(at time.Duration, attrs map[string]string) models.LogRecord {
		r := makeLogRecord(projectId, 9, "telegram sent", base.Add(at))
		r.LogAttributes = attrs
		r.ResourceAttributes = map[string]string{"host.name": "plc-1"}
		return r
	}
	records := []models.LogRecord{
		record(5*time.Second, map[string]string{"TimerSend": "10", "Label": "a"}),
		record(10*time.Second, map[string]string{"TimerSend": "20", "Label": "a"}),
		record(15*time.Second, map[string]string{"TimerSend": "30", "Label": "b"}),
		record(65*time.Second, map[string]string{"TimerSend": "5", "Label": "a"}),
		record(70*time.Second, map[string]string{"TimerSend": "-2.5", "Label": "b"}),
		record(75*time.Second, map[string]string{"TimerSend": "1e2", "Label": "b"}),
		record(125*time.Second, map[string]string{"TimerSend": "abc"}),
		record(130*time.Second, map[string]string{"TimerSend": ""}),
		record(135*time.Second, map[string]string{"TimerSend": "007"}),
		record(140*time.Second, map[string]string{"TimerSend": " 12"}),
		record(145*time.Second, map[string]string{"Label": "a"}),
		record(150*time.Second, nil),
	}
	other := makeLogRecord(uuid.New(), 9, "other project", base.Add(5*time.Second))
	other.LogAttributes = map[string]string{"TimerSend": "999"}
	records = append(records, other)
	if err := LogRecordRepository.InsertAsync(ctx, records); err != nil {
		t.Fatalf("InsertAsync: %v", err)
	}
	return chartFixture{
		ctx:       ctx,
		projectId: projectId,
		base:      base,
		params:    shared.LogSearchParams{ProjectId: projectId, FromDate: base, ToDate: base.Add(5*time.Minute - time.Millisecond)},
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestLogRecordRepository_ExistsAttributeFilter(t *testing.T) {
	f := seedChartLogs(t)
	count := func(filter LogAttributeFilter) int64 {
		params := f.params
		params.AttributeFilters = []LogAttributeFilter{filter}
		params.Page, params.PageSize = 1, 50
		_, total, err := LogRecordRepository.Search(f.ctx, params)
		if err != nil {
			t.Fatalf("Search (%+v): %v", filter, err)
		}
		return total
	}
	if got := count(LogAttributeFilter{Scope: "log", Key: "TimerSend", Exists: true}); got != 10 {
		t.Errorf("TimerSend exists matched %d logs, want 10 (an empty value still carries the key)", got)
	}
	if got := count(LogAttributeFilter{Scope: "log", Key: "TimerSend", Exists: true, Exclude: true}); got != 2 {
		t.Errorf("TimerSend does not exist matched %d logs, want 2", got)
	}
	// The value is ignored: exists is about the key alone.
	if got := count(LogAttributeFilter{Scope: "log", Key: "Label", Value: "zzz", Exists: true}); got != 7 {
		t.Errorf("Label exists matched %d logs, want 7", got)
	}
	if got := count(LogAttributeFilter{Scope: "resource", Key: "host.name", Exists: true}); got != 12 {
		t.Errorf("resource host.name exists matched %d logs, want 12", got)
	}
	if got := count(LogAttributeFilter{Scope: "resource", Key: "TimerSend", Exists: true}); got != 0 {
		t.Errorf("resource TimerSend exists matched %d logs, want 0 (it is a log attribute)", got)
	}
}

func TestLogRecordRepository_AttributeStatsPerBucket(t *testing.T) {
	f := seedChartLogs(t)
	rows, err := LogRecordRepository.AttributeStats(f.ctx, f.params, LogAttributeTarget{Scope: "log", Key: "TimerSend"}, 60)
	if err != nil {
		t.Fatalf("AttributeStats: %v", err)
	}
	buckets, present, numeric := shared.BuildChartSeries(rows)
	if present != 10 || numeric != 6 {
		t.Fatalf("present=%d numeric=%d, want 10 and 6 (\"abc\", \"\", \"007\" and \" 12\" are not JSON numbers)", present, numeric)
	}
	if len(buckets) != 2 {
		t.Fatalf("got %d buckets, want 2 (the third minute holds no number): %+v", len(buckets), buckets)
	}
	first, second := buckets[0], buckets[1]
	if !first.Start.Equal(f.base) || !second.Start.Equal(f.base.Add(time.Minute)) {
		t.Errorf("bucket starts %v and %v, want %v and a minute later", first.Start, second.Start, f.base)
	}
	if first.Count != 3 || !near(first.Sum, 60) || !near(first.Avg, 20) || !near(first.Min, 10) || !near(first.Max, 30) {
		t.Errorf("first bucket = %+v, want count 3, sum 60, avg 20, min 10, max 30", first)
	}
	// Linear interpolation between the sorted values, as quantile_cont does:
	// p95 of 10, 20, 30 sits at position 1.9.
	if !near(first.P50, 20) || !near(first.P95, 29) || !near(first.P99, 29.8) {
		t.Errorf("first bucket percentiles = %v %v %v, want 20 29 29.8", first.P50, first.P95, first.P99)
	}
	if second.Count != 3 || !near(second.Sum, 102.5) || !near(second.Min, -2.5) || !near(second.Max, 100) || !near(second.P50, 5) {
		t.Errorf("second bucket = %+v, want count 3, sum 102.5, min -2.5, max 100, p50 5", second)
	}

	filtered := f.params
	filtered.AttributeFilters = []LogAttributeFilter{{Scope: "log", Key: "Label", Value: "b"}}
	rows, err = LogRecordRepository.AttributeStats(f.ctx, filtered, LogAttributeTarget{Scope: "log", Key: "TimerSend"}, 60)
	if err != nil {
		t.Fatalf("AttributeStats with a filter: %v", err)
	}
	buckets, _, numeric = shared.BuildChartSeries(rows)
	if numeric != 3 || len(buckets) != 2 || !near(buckets[0].Max, 30) || !near(buckets[1].Sum, 97.5) {
		t.Errorf("Label=b: numeric=%d buckets=%+v, want the three values labelled b", numeric, buckets)
	}

	rows, err = LogRecordRepository.AttributeStats(f.ctx, f.params, LogAttributeTarget{Scope: "log", Key: "Missing"}, 60)
	if err != nil {
		t.Fatalf("AttributeStats of a missing key: %v", err)
	}
	if buckets, present, _ := shared.BuildChartSeries(rows); present != 0 || len(buckets) != 0 {
		t.Errorf("missing key: present=%d buckets=%+v, want nothing", present, buckets)
	}
}

func TestLogRecordRepository_AttributeStatsWholeRangeAndBins(t *testing.T) {
	f := seedChartLogs(t)
	target := LogAttributeTarget{Scope: "log", Key: "TimerSend"}
	rows, err := LogRecordRepository.AttributeStats(f.ctx, f.params, target, 0)
	if err != nil {
		t.Fatalf("AttributeStats: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows for the whole range, want 1: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.Present != 10 || row.Numeric != 6 || row.Integral != 5 {
		t.Errorf("present=%d numeric=%d integral=%d, want 10, 6 and 5 (-2.5 is not whole)", row.Present, row.Numeric, row.Integral)
	}
	// Sorted: -2.5 5 10 20 30 100. p50 sits halfway between 10 and 20.
	if !near(row.Min, -2.5) || !near(row.Max, 100) || !near(row.Sum, 162.5) || !near(row.P50, 15) {
		t.Errorf("whole range = %+v, want min -2.5, max 100, sum 162.5, p50 15", row)
	}

	binning := shared.NiceChartBins(row.Min, row.Max, 5, false)
	if binning.Start != -25 || binning.Width != 25 || binning.Count != 6 {
		t.Fatalf("binning = %+v, want 6 bins of 25 from -25", binning)
	}
	binRows, err := LogRecordRepository.AttributeBins(f.ctx, f.params, target, binning)
	if err != nil {
		t.Fatalf("AttributeBins: %v", err)
	}
	bins := shared.BuildChartBins(binning, binRows)
	want := []int64{1, 3, 1, 0, 0, 1}
	for i, b := range bins {
		if b.Count != want[i] {
			t.Errorf("bin %d [%v, %v) = %d, want %d", i, b.Start, b.End, b.Count, want[i])
		}
	}
}
