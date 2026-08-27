//go:build !telemetry_ch && !telemetry_duckdb

package migrations

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// Every per-group telemetry read pairs a group column with the table's time
// column: as a second filter on the grouped lists, as the ORDER BY on the
// per-group detail reads. An index that stops at the group column leaves SQLite
// choosing between two bad plans -- seek the time index and re-filter the group
// per row, which is a whole-window scan per group on the lists and a whole-table
// walk on a detail read whose group holds fewer rows than the LIMIT; or seek the
// group and sort its entire history. Which one it picks flips on whether ANALYZE
// has ever run, so both are reachable in production and neither shows up in a
// functional test. Guard the covering indexes: a later migration must not narrow
// or drop one unnoticed.
func TestTelemetryGroupIndexesCoverTimeColumn(t *testing.T) {
	telemetryDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	defer telemetryDB.Close()
	telemetryDB.SetMaxOpenConns(1)

	if err := runMigrationsOn(telemetryDB, migrationsSqliteTelemetryFS, "sqlite_telemetry", "schema_migrations", sqliteTrackingDDL); err != nil {
		t.Fatalf("telemetry migrations failed: %v", err)
	}

	for _, tc := range []struct{ table, group, timeCol string }{
		{"endpoints", "endpoint", "recorded_at"},
		{"tasks", "task_name", "recorded_at"},
		{"ai_traces", "trace_name", "recorded_at"},
		{"exception_stack_traces", "exception_hash", "recorded_at"},
		{"check_results", "check_id", "recorded_at"},
		{"log_records", "service_name", "timestamp"},
		{"metric_points", "name", "recorded_at"},
	} {
		var covered bool
		if err := telemetryDB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM pragma_index_list(?1) AS il
				WHERE (SELECT name FROM pragma_index_info(il.name) WHERE seqno = 0) = 'project_id'
				  AND (SELECT name FROM pragma_index_info(il.name) WHERE seqno = 1) = ?2
				  AND (SELECT name FROM pragma_index_info(il.name) WHERE seqno = 2) = ?3
			)`, tc.table, tc.group, tc.timeCol).Scan(&covered); err != nil {
			t.Fatalf("%s: failed to inspect indexes: %v", tc.table, err)
		}
		if !covered {
			t.Errorf("%s: no index leading with (project_id, %s, %s); per-group reads fall back to scanning the table or sorting the whole group", tc.table, tc.group, tc.timeCol)
		}
	}
}
