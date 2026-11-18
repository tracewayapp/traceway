//go:build !telemetry_duckdb

package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
)

// DuckDB is left out: its VARCHAR refuses invalid UTF-8, so its old tables cannot hold a name the span store rejects.
func TestMoveOverRejectedSpanDoesNotCompleteDay(t *testing.T) {
	setupTestDB(t)
	setupMoveOverProgress(t)
	legacyReset(t)
	t.Cleanup(func() { legacyReset(t) })
	at := time.Now().UTC().Truncate(moveOverDay).Add(10 * time.Hour)
	seedLegacyDay(t, uuid.New(), at, uuid.New(), "GET /invalid-utf8-\xff")
	if err := RunMoveOver(context.Background(), MoveOverOptions{Log: func(string, ...any) {}}); err == nil {
		t.Fatal("rejected canonical span was reported as a successful migration")
	}
	progress, err := (moveOverProgress{}).FindProgress(context.Background())
	if err != nil || len(progress) != 1 || progress[0].State != transactional.MoveOverStarted {
		t.Fatalf("rejected day must stay started: %+v, %v", progress, err)
	}
	if moveOverRowCount(t, "endpoints_v2") != 0 {
		t.Fatal("rejected root must not leave an endpoint projection")
	}
}
