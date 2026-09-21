package telemetry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func makeLogRecord(projectId uuid.UUID, severity uint8, body string, timestamp time.Time) models.LogRecord {
	return models.LogRecord{
		Id:             uuid.New(),
		ProjectId:      projectId,
		Timestamp:      timestamp,
		SeverityText:   "test",
		SeverityNumber: severity,
		ServiceName:    "svc",
		Body:           body,
	}
}

func TestLogRecordRepository_SearchPaginationTotal(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	records := []models.LogRecord{
		makeLogRecord(projectId, 17, "error one", now),
		makeLogRecord(projectId, 17, "error two", now.Add(time.Second)),
		makeLogRecord(projectId, 9, "info one", now.Add(2*time.Second)),
	}
	if err := LogRecordRepository.InsertAsync(ctx, records); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	base := shared.LogSearchParams{
		ProjectId: projectId,
		FromDate:  now.Add(-time.Hour),
		ToDate:    now.Add(time.Hour),
		PageSize:  2,
	}

	params := base
	params.Page = 1
	rows, total, err := LogRecordRepository.Search(ctx, params)
	if err != nil {
		t.Fatalf("Search page 1 failed: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows on page 1, got %d", len(rows))
	}

	params.Page = 2
	rows, total, err = LogRecordRepository.Search(ctx, params)
	if err != nil {
		t.Fatalf("Search page 2 failed: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3 on page 2, got %d", total)
	}
	if len(rows) != 1 {
		t.Errorf("expected 1 row on page 2, got %d", len(rows))
	}

	// A page past the last row must still report the full total.
	params.Page = 5
	rows, total, err = LogRecordRepository.Search(ctx, params)
	if err != nil {
		t.Fatalf("Search page-past-end failed: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3 on page past end, got %d", total)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows on page past end, got %d", len(rows))
	}

	params = base
	params.Page = 1
	params.MinSeverity = 17
	rows, total, err = LogRecordRepository.Search(ctx, params)
	if err != nil {
		t.Fatalf("Search severity filter failed: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2 with severity filter, got %d", total)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows with severity filter, got %d", len(rows))
	}

	params.MinSeverity = 30
	rows, total, err = LogRecordRepository.Search(ctx, params)
	if err != nil {
		t.Fatalf("Search no-match failed: %v", err)
	}
	if total != 0 || len(rows) != 0 {
		t.Errorf("expected no rows and total 0, got %d rows total %d", len(rows), total)
	}
}

func searchBodies(t *testing.T, params shared.LogSearchParams) ([]string, int64) {
	t.Helper()
	rows, total, err := LogRecordRepository.Search(context.Background(), params)
	if err != nil {
		t.Fatalf("Search page %d failed: %v", params.Page, err)
	}
	bodies := make([]string, 0, len(rows))
	for _, r := range rows {
		bodies = append(bodies, r.Body)
	}
	return bodies, total
}

func assertBodies(t *testing.T, label string, got, want []string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s: expected %v, got %v", label, want, got)
	}
}

func TestLogRecordRepository_SearchByTraceIdsMergesAndPaginates(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	var records []models.LogRecord
	for i := 0; i < 7; i++ {
		rec := makeLogRecord(projectId, 9, fmt.Sprintf("row %d", i), now.Add(time.Duration(i)*time.Second))
		rec.TraceId = []string{"aaaa", "bbbb"}[i%2]
		records = append(records, rec)
	}
	unrelated := makeLogRecord(projectId, 9, "unrelated", now.Add(10*time.Second))
	unrelated.TraceId = "cccc"
	records = append(records, unrelated)
	if err := LogRecordRepository.InsertAsync(ctx, records); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	base := shared.LogSearchParams{
		ProjectId: projectId,
		FromDate:  now.Add(-time.Hour),
		ToDate:    now.Add(time.Hour),
		TraceIds:  []string{"aaaa", "bbbb", "dddd"},
		PageSize:  3,
	}

	var got []string
	for page := 1; page <= 3; page++ {
		params := base
		params.Page = page
		bodies, total := searchBodies(t, params)
		if total != 7 {
			t.Errorf("page %d: expected total 7, got %d", page, total)
		}
		got = append(got, bodies...)
	}
	assertBodies(t, "desc pages", got, []string{"row 6", "row 5", "row 4", "row 3", "row 2", "row 1", "row 0"})

	params := base
	params.Page = 2
	params.SortDirection = "asc"
	bodies, _ := searchBodies(t, params)
	assertBodies(t, "asc page 2", bodies, []string{"row 3", "row 4", "row 5"})

	params = base
	params.Page = 1
	params.TraceIds = []string{"dddd"}
	bodies, total := searchBodies(t, params)
	if total != 0 || len(bodies) != 0 {
		t.Errorf("unknown trace id: expected no rows, got total %d rows %v", total, bodies)
	}
}

func TestLogRecordRepository_SearchByTraceIdsPastBranchCap(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	now := truncateMs(time.Now().UTC())

	records := []models.LogRecord{
		makeLogRecord(projectId, 9, "older", now),
		makeLogRecord(projectId, 9, "newer", now.Add(time.Second)),
		makeLogRecord(projectId, 9, "unrelated", now.Add(2*time.Second)),
	}
	records[0].TraceId = "aaaa"
	records[1].TraceId = "aaaa"
	records[2].TraceId = "cccc"
	if err := LogRecordRepository.InsertAsync(ctx, records); err != nil {
		t.Fatalf("InsertAsync failed: %v", err)
	}

	traceIds := make([]string, 0, 300)
	for i := 0; i < 299; i++ {
		traceIds = append(traceIds, fmt.Sprintf("%04x", i))
	}
	traceIds = append(traceIds, "aaaa")

	params := shared.LogSearchParams{
		ProjectId: projectId,
		FromDate:  now.Add(-time.Hour),
		ToDate:    now.Add(time.Hour),
		TraceIds:  traceIds,
		PageSize:  1,
		Page:      1,
	}
	bodies, total := searchBodies(t, params)
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	assertBodies(t, "page 1", bodies, []string{"newer"})

	params.Page = 2
	bodies, _ = searchBodies(t, params)
	assertBodies(t, "page 2", bodies, []string{"older"})
}

func TestLogRecordRepository_WholeTracePaginatesAcrossProjects(t *testing.T) {
	setupTestDB(t)
	projects := []uuid.UUID{uuid.New(), uuid.New()}
	other := uuid.New()
	now := truncateMs(time.Now().UTC())
	const trace = "0123456789abcdef0123456789abcdef"
	var records []models.LogRecord
	for i := 0; i < 7; i++ {
		record := makeLogRecord(projects[i%2], 9, fmt.Sprintf("row %d", i), now.Add(time.Duration(i)*time.Second))
		record.TraceId = trace
		records = append(records, record)
	}
	outside := makeLogRecord(other, 9, "outside", now)
	outside.TraceId = trace
	records = append(records, outside, makeLogRecord(projects[0], 9, "another trace", now))
	if err := LogRecordRepository.InsertAsync(context.Background(), records); err != nil {
		t.Fatal(err)
	}
	params := shared.LogSearchParams{ProjectId: projects[0], ProjectIds: projects, TraceId: trace,
		FromDate: now.Add(-time.Hour), ToDate: now.Add(time.Hour), OrderBy: "timestamp", SortDirection: "asc", PageSize: 3}
	var got []string
	for page := 1; page <= 3; page++ {
		params.Page = page
		bodies, total := searchBodies(t, params)
		if total != 7 {
			t.Fatalf("page %d: total = %d", page, total)
		}
		got = append(got, bodies...)
	}
	assertBodies(t, "whole trace", got, []string{"row 0", "row 1", "row 2", "row 3", "row 4", "row 5", "row 6"})
	params.ProjectIds = nil
	params.Page, params.PageSize = 1, 10
	bodies, total := searchBodies(t, params)
	if total != 4 {
		t.Fatalf("project scope widened: %d", total)
	}
	assertBodies(t, "selected project", bodies, []string{"row 0", "row 2", "row 4", "row 6"})
}
