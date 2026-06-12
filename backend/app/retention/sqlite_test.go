//go:build !pgch

package retention

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	_ "modernc.org/sqlite"
)

func setupRetentionTestDB(t *testing.T) {
	t.Helper()

	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open main: %v", err)
	}
	mainDB.SetMaxOpenConns(1)

	telDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open telemetry: %v", err)
	}
	telDB.SetMaxOpenConns(1)

	for _, tgt := range telemetryRetentionTargets {
		ddl := fmt.Sprintf(
			"CREATE TABLE %s (id TEXT, project_id TEXT, %s DATETIME NOT NULL)",
			tgt.table, tgt.column,
		)
		if _, err := telDB.Exec(ddl); err != nil {
			t.Fatalf("telemetry ddl %s: %v", tgt.table, err)
		}
	}

	prevDB, prevTelDB, prevDriver := db.DB, db.TelemetryDB, db.Driver
	db.DB = mainDB
	db.TelemetryDB = telDB
	db.Driver = lit.SQLite

	t.Cleanup(func() {
		mainDB.Close()
		telDB.Close()
		db.DB = prevDB
		db.TelemetryDB = prevTelDB
		db.Driver = prevDriver
	})
}

func insertWithTime(t *testing.T, ex *sql.DB, table, column string, when time.Time) {
	t.Helper()
	ts := when.UTC().Format(time.RFC3339Nano)
	stmt := fmt.Sprintf("INSERT INTO %s (project_id, %s) VALUES (?, ?)", table, column)
	if _, err := ex.Exec(stmt, uuid.NewString(), ts); err != nil {
		t.Fatalf("insert into %s: %v", table, err)
	}
}

func countRows(t *testing.T, ex *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := ex.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestRunSQLiteRetention_DeletesOldKeepsFresh(t *testing.T) {
	setupRetentionTestDB(t)

	now := time.Now().UTC()
	old := now.AddDate(0, 0, -45)
	fresh := now.AddDate(0, 0, -5)

	insertWithTime(t, db.TelemetryDB, "endpoints", "recorded_at", old)
	insertWithTime(t, db.TelemetryDB, "endpoints", "recorded_at", old)
	insertWithTime(t, db.TelemetryDB, "endpoints", "recorded_at", fresh)
	insertWithTime(t, db.TelemetryDB, "endpoints", "recorded_at", fresh)
	insertWithTime(t, db.TelemetryDB, "endpoints", "recorded_at", fresh)

	insertWithTime(t, db.TelemetryDB, "log_records", "timestamp", old)
	insertWithTime(t, db.TelemetryDB, "log_records", "timestamp", fresh)
	insertWithTime(t, db.TelemetryDB, "log_records", "timestamp", fresh)

	insertWithTime(t, db.TelemetryDB, "sessions", "started_at", old)
	insertWithTime(t, db.TelemetryDB, "sessions", "started_at", fresh)

	insertWithTime(t, db.TelemetryDB, "fired_notifications", "fired_at", old)
	insertWithTime(t, db.TelemetryDB, "fired_notifications", "fired_at", fresh)

	runSQLiteRetention(context.Background(), 30)

	assertCount(t, db.TelemetryDB, "endpoints", 3)
	assertCount(t, db.TelemetryDB, "log_records", 2)
	assertCount(t, db.TelemetryDB, "sessions", 1)
	assertCount(t, db.TelemetryDB, "fired_notifications", 1)
}

func TestRunSQLiteRetention_BoundaryRowAtCutoffStays(t *testing.T) {
	setupRetentionTestDB(t)

	cutoff := time.Now().UTC().AddDate(0, 0, -30)

	insertWithTime(t, db.TelemetryDB, "endpoints", "recorded_at", cutoff.Add(-time.Second))
	insertWithTime(t, db.TelemetryDB, "endpoints", "recorded_at", cutoff.Add(time.Second))

	runSQLiteRetention(context.Background(), 30)

	assertCount(t, db.TelemetryDB, "endpoints", 1)
}

func TestRunSQLiteRetention_AlreadyCancelledContextIsNoOp(t *testing.T) {
	setupRetentionTestDB(t)

	old := time.Now().UTC().AddDate(0, 0, -45)
	insertWithTime(t, db.TelemetryDB, "endpoints", "recorded_at", old)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runSQLiteRetention(ctx, 30)

	assertCount(t, db.TelemetryDB, "endpoints", 1)
}

func assertCount(t *testing.T, ex *sql.DB, table string, want int) {
	t.Helper()
	if got := countRows(t, ex, table); got != want {
		t.Errorf("%s: got %d rows after cleanup, want %d", table, got, want)
	}
}
