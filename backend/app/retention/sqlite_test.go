//go:build !telemetry_ch && !telemetry_firebolt

package retention

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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

func TestRunSQLiteRetention_PrunesProfilingTables(t *testing.T) {
	setupRetentionTestDB(t)

	profilingTables := []struct {
		table  string
		column string
	}{
		{"profiling_samples", "start_time"},
		{"profiles", "recorded_at"},
		{"profiling_stacks", "last_seen"},
	}
	for _, pt := range profilingTables {
		ddl := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s (id TEXT, project_id TEXT, %s DATETIME NOT NULL)",
			pt.table, pt.column,
		)
		if _, err := db.TelemetryDB.Exec(ddl); err != nil {
			t.Fatalf("create %s: %v", pt.table, err)
		}
	}

	now := time.Now().UTC()
	old := now.AddDate(0, 0, -45)
	fresh := now.AddDate(0, 0, -5)

	for _, pt := range profilingTables {
		insertWithTime(t, db.TelemetryDB, pt.table, pt.column, old)
		insertWithTime(t, db.TelemetryDB, pt.table, pt.column, fresh)
	}

	runSQLiteRetention(context.Background(), 30)

	for _, pt := range profilingTables {
		assertCount(t, db.TelemetryDB, pt.table, 1)
	}
}

func TestRunSQLiteRetention_TruncatesWALAfterPass(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.db")
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=auto_vacuum(incremental)&_pragma=wal_autocheckpoint(1000000)"

	telDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open telemetry: %v", err)
	}
	telDB.SetMaxOpenConns(1)
	if err := telDB.Ping(); err != nil {
		t.Fatalf("ping telemetry: %v", err)
	}

	for _, tgt := range telemetryRetentionTargets {
		ddl := fmt.Sprintf("CREATE TABLE %s (id TEXT, project_id TEXT, %s DATETIME NOT NULL)", tgt.table, tgt.column)
		if _, err := telDB.Exec(ddl); err != nil {
			t.Fatalf("ddl %s: %v", tgt.table, err)
		}
	}

	prevTelDB, prevDriver := db.TelemetryDB, db.Driver
	db.TelemetryDB = telDB
	db.Driver = lit.SQLite
	t.Cleanup(func() {
		telDB.Close()
		db.TelemetryDB = prevTelDB
		db.Driver = prevDriver
	})

	old := time.Now().UTC().AddDate(0, 0, -45)
	for range 2000 {
		insertWithTime(t, telDB, "endpoints", "recorded_at", old)
	}

	walPath := path + "-wal"
	info, err := os.Stat(walPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("expected WAL to grow before retention (err=%v)", err)
	}

	runSQLiteRetention(context.Background(), 30)

	assertCount(t, telDB, "endpoints", 0)

	if info, err := os.Stat(walPath); err == nil && info.Size() > 0 {
		t.Errorf("expected WAL truncated to 0 after retention, got %d bytes", info.Size())
	}
}

func assertCount(t *testing.T, ex *sql.DB, table string, want int) {
	t.Helper()
	if got := countRows(t, ex, table); got != want {
		t.Errorf("%s: got %d rows after cleanup, want %d", table, got, want)
	}
}
