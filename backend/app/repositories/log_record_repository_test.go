package repositories

import (
	"context"
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
