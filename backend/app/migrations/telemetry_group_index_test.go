//go:build !telemetry_ch && !telemetry_duckdb

package migrations

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// The grouped-list reads for these three tables filter
// (project_id, <group>, recorded_at). An index that stops at the group column
// leaves SQLite to seek the (project_id, recorded_at) index and re-filter the
// group column across the whole window once per group, which is quadratic in
// the number of groups and invisible to functional tests. Guard the covering
// indexes so a later migration cannot narrow or drop them unnoticed.
func TestTelemetryGroupIndexesCoverRecordedAt(t *testing.T) {
	telemetryDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	defer telemetryDB.Close()
	telemetryDB.SetMaxOpenConns(1)

	if err := runMigrationsOn(telemetryDB, migrationsSqliteTelemetryFS, "sqlite_telemetry", "schema_migrations", sqliteTrackingDDL); err != nil {
		t.Fatalf("telemetry migrations failed: %v", err)
	}

	for _, tc := range []struct{ table, group string }{
		{"endpoints", "endpoint"},
		{"tasks", "task_name"},
		{"ai_traces", "trace_name"},
	} {
		var covered bool
		if err := telemetryDB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM pragma_index_list(?1) AS il
				WHERE (SELECT name FROM pragma_index_info(il.name) WHERE seqno = 0) = 'project_id'
				  AND (SELECT name FROM pragma_index_info(il.name) WHERE seqno = 1) = ?2
				  AND (SELECT name FROM pragma_index_info(il.name) WHERE seqno = 2) = 'recorded_at'
			)`, tc.table, tc.group).Scan(&covered); err != nil {
			t.Fatalf("%s: failed to inspect indexes: %v", tc.table, err)
		}
		if !covered {
			t.Errorf("%s: no index leading with (project_id, %s, recorded_at); per-group queries will scan the whole window once per group", tc.table, tc.group)
		}
	}
}
