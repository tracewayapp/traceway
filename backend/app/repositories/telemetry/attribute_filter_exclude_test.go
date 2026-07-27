//go:build !telemetry_ch

package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestLogRecordRepository_Search_ExcludeAttributeFilter(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	record := func(attrs map[string]string) models.LogRecord {
		return models.LogRecord{
			Id:            uuid.New(),
			ProjectId:     projectId,
			Timestamp:     base,
			ServiceName:   "checkout",
			Body:          "request handled",
			LogAttributes: attrs,
		}
	}
	if err := LogRecordRepository.InsertAsync(ctx, []models.LogRecord{
		record(map[string]string{"http.route": "GET /checkout"}),
		record(map[string]string{"http.route": "GET /cart"}),
		record(map[string]string{"other.key": "x"}),
	}); err != nil {
		t.Fatalf("InsertAsync: %v", err)
	}

	records, total, err := LogRecordRepository.Search(ctx, LogSearchParams{
		ProjectId:        projectId,
		FromDate:         base.Add(-time.Hour),
		ToDate:           base.Add(time.Hour),
		AttributeFilters: []LogAttributeFilter{{Scope: "log", Key: "http.route", Value: "GET /checkout", Exclude: true}},
	})
	if err != nil {
		t.Fatalf("Search (exclude attribute filter): %v", err)
	}
	// The excluded row is dropped; the row that doesn't carry the attribute at
	// all must survive the exclusion.
	if total != 2 || len(records) != 2 {
		t.Fatalf("exclude attribute filter returned %d rows (total=%d), want 2", len(records), total)
	}
	for _, r := range records {
		if r.LogAttributes["http.route"] == "GET /checkout" {
			t.Errorf("excluded record (http.route=GET /checkout) was returned")
		}
	}
}
