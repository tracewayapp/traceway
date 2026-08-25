//go:build !telemetry_ch && !telemetry_firebolt

package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestLogRecordRepository_Search_ContainsAttributeFilter(t *testing.T) {
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

	search := func(filter LogAttributeFilter) ([]models.LogRecord, int64) {
		records, total, err := LogRecordRepository.Search(ctx, LogSearchParams{
			ProjectId:        projectId,
			FromDate:         base.Add(-time.Hour),
			ToDate:           base.Add(time.Hour),
			AttributeFilters: []LogAttributeFilter{filter},
		})
		if err != nil {
			t.Fatalf("Search (%+v): %v", filter, err)
		}
		return records, total
	}

	// Case-insensitive substring match.
	records, total := search(LogAttributeFilter{Scope: "log", Key: "http.route", Value: "Checkout", Contains: true})
	if total != 1 || len(records) != 1 {
		t.Fatalf("contains attribute filter returned %d rows (total=%d), want 1", len(records), total)
	}
	if records[0].LogAttributes["http.route"] != "GET /checkout" {
		t.Errorf("contains attribute filter returned http.route=%q, want GET /checkout", records[0].LogAttributes["http.route"])
	}

	// Negated form drops the match; the row that doesn't carry the attribute at
	// all must survive.
	records, total = search(LogAttributeFilter{Scope: "log", Key: "http.route", Value: "Checkout", Contains: true, Exclude: true})
	if total != 2 || len(records) != 2 {
		t.Fatalf("not-contains attribute filter returned %d rows (total=%d), want 2", len(records), total)
	}
	for _, r := range records {
		if r.LogAttributes["http.route"] == "GET /checkout" {
			t.Errorf("excluded record (http.route=GET /checkout) was returned")
		}
	}
}
