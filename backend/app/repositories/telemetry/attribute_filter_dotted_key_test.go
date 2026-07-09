//go:build !telemetry_ch

package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestLogRecordRepository_Search_FiltersByDottedAttributeKey(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	record := func(route string) models.LogRecord {
		return models.LogRecord{
			Id:            uuid.New(),
			ProjectId:     projectId,
			Timestamp:     base,
			ServiceName:   "checkout",
			Body:          "request handled",
			LogAttributes: map[string]string{"http.route": route},
		}
	}
	if err := LogRecordRepository.InsertAsync(ctx, []models.LogRecord{
		record("GET /checkout"),
		record("GET /cart"),
	}); err != nil {
		t.Fatalf("InsertAsync: %v", err)
	}

	records, total, err := LogRecordRepository.Search(ctx, LogSearchParams{
		ProjectId:        projectId,
		FromDate:         base.Add(-time.Hour),
		ToDate:           base.Add(time.Hour),
		AttributeFilters: []LogAttributeFilter{{Scope: "log", Key: "http.route", Value: "GET /checkout"}},
	})
	if err != nil {
		t.Fatalf("Search (dotted attribute filter): %v", err)
	}
	if total != 1 || len(records) != 1 {
		t.Fatalf("dotted log attribute filter returned %d rows (total=%d), want 1 (dotted key must match in SQLite mode)", len(records), total)
	}
	if got := records[0].LogAttributes["http.route"]; got != "GET /checkout" {
		t.Errorf("matched record http.route = %q, want GET /checkout", got)
	}
}

func TestSessionRepository_FindAll_FiltersByDottedAttributeKey(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	projectId := uuid.New()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	session := func(route string) models.Session {
		return models.Session{
			Id:         uuid.New(),
			ProjectId:  projectId,
			StartedAt:  base,
			Attributes: map[string]string{"http.route": route},
		}
	}
	if err := SessionRepository.Upsert(ctx, []models.Session{
		session("GET /checkout"),
		session("GET /cart"),
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	sessions, total, err := SessionRepository.FindAll(ctx, projectId, base.Add(-time.Hour), base.Add(time.Hour), 1, 50, "started_at", "desc", "",
		[]SessionAttributeFilter{{Key: "http.route", Value: "GET /checkout"}})
	if err != nil {
		t.Fatalf("FindAll (dotted attribute filter): %v", err)
	}
	if total != 1 || len(sessions) != 1 {
		t.Fatalf("dotted session attribute filter returned %d rows (total=%d), want 1 (dotted key must match in SQLite mode)", len(sessions), total)
	}
	if got := sessions[0].Attributes["http.route"]; got != "GET /checkout" {
		t.Errorf("matched session http.route = %q, want GET /checkout", got)
	}
}
