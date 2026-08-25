//go:build !telemetry_ch && !telemetry_firebolt

package retention

import (
	"context"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
)

func TestRunLogRecordsCap_DeletesOldestBeyondCap(t *testing.T) {
	setupRetentionTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	for i := range 15 {
		insertWithTime(t, db.TelemetryDB, "log_records", "timestamp", now.Add(time.Duration(-i)*time.Second))
	}

	runLogRecordsCap(context.Background(), 10)

	assertCount(t, db.TelemetryDB, "log_records", 10)

	// The 10 newest rows (offsets 0..-9s) survive; the boundary is -9s.
	boundary := now.Add(-9 * time.Second).Format(time.RFC3339Nano)
	var older int
	if err := db.TelemetryDB.QueryRow("SELECT count(*) FROM log_records WHERE timestamp < ?", boundary).Scan(&older); err != nil {
		t.Fatalf("count older: %v", err)
	}
	if older != 0 {
		t.Errorf("expected 0 rows older than the cap boundary, got %d", older)
	}
}

func TestRunLogRecordsCap_UnderCapIsNoOp(t *testing.T) {
	setupRetentionTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	for i := range 5 {
		insertWithTime(t, db.TelemetryDB, "log_records", "timestamp", now.Add(time.Duration(-i)*time.Second))
	}

	runLogRecordsCap(context.Background(), 10)

	assertCount(t, db.TelemetryDB, "log_records", 5)
}

func TestRunLogRecordsCap_ExactlyAtCapIsNoOp(t *testing.T) {
	setupRetentionTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	for i := range 10 {
		insertWithTime(t, db.TelemetryDB, "log_records", "timestamp", now.Add(time.Duration(-i)*time.Second))
	}

	runLogRecordsCap(context.Background(), 10)

	assertCount(t, db.TelemetryDB, "log_records", 10)
}

func TestRunLogRecordsCap_AlreadyCancelledContextIsNoOp(t *testing.T) {
	setupRetentionTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	for i := range 15 {
		insertWithTime(t, db.TelemetryDB, "log_records", "timestamp", now.Add(time.Duration(-i)*time.Second))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runLogRecordsCap(ctx, 10)

	assertCount(t, db.TelemetryDB, "log_records", 15)
}
