//go:build !telemetry_ch && !telemetry_duckdb

package migrations

import (
	"database/sql"
	"slices"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/retention"
	_ "modernc.org/sqlite"
)

// Every per-group telemetry read pairs a group column with the table's time
// column: as a second filter on the grouped lists, as the ORDER BY on the
// per-group detail reads. An index that stops at the group column leaves SQLite
// to either re-filter the group across the whole window once per group, which
// is quadratic in the number of groups, or seek the group and sort its entire
// history. Both are unbounded and invisible to functional tests. Guard the
// covering indexes so a later migration cannot narrow or drop one unnoticed.
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

	timeColumn := retention.TelemetryTimeColumns()
	indexes := createdIndexes(t, telemetryDB)

	for _, tc := range []struct{ table, group string }{
		{"endpoints", "endpoint"},
		{"tasks", "task_name"},
		{"ai_traces", "trace_name"},
		{"exception_stack_traces", "session_id"},
		{"log_records", "trace_id"},
	} {
		timeCol, pruned := timeColumn[tc.table]
		if !pruned {
			t.Fatalf("%s is not a retention target, so it has no time column to guard", tc.table)
		}
		want := []string{"project_id", tc.group, timeCol}
		covered := slices.ContainsFunc(indexes, func(ix telemetryIndex) bool {
			return ix.table == tc.table && len(ix.cols) >= len(want) && slices.Equal(ix.cols[:len(want)], want)
		})
		if !covered {
			t.Errorf("%s: no index leading with (%s)", tc.table, strings.Join(want, ", "))
		}
	}

}

type telemetryIndex struct {
	table, name string
	cols        []string
}

// createdIndexes lists every CREATE INDEX index with its columns in order. An
// expression column has no name and is listed as "". UNIQUE and PRIMARY KEY
// autoindexes are left out: their column set is dictated by the constraint,
// not by a read pattern.
func createdIndexes(t *testing.T, telemetryDB *sql.DB) []telemetryIndex {
	t.Helper()
	rows, err := telemetryDB.Query(`
		SELECT m.name, il.name, COALESCE(ii.name, '') FROM sqlite_master AS m
		JOIN pragma_index_list(m.name) AS il
		JOIN pragma_index_info(il.name) AS ii
		WHERE m.type = 'table' AND il.origin = 'c'
		ORDER BY m.name, il.name, ii.seqno`)
	if err != nil {
		t.Fatalf("failed to list indexes: %v", err)
	}
	defer rows.Close()

	var indexes []telemetryIndex
	for rows.Next() {
		var table, index, col string
		if err := rows.Scan(&table, &index, &col); err != nil {
			t.Fatalf("failed to scan index column: %v", err)
		}
		if n := len(indexes); n == 0 || indexes[n-1].name != index {
			indexes = append(indexes, telemetryIndex{table: table, name: index})
		}
		indexes[len(indexes)-1].cols = append(indexes[len(indexes)-1].cols, col)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed to iterate indexes: %v", err)
	}
	return indexes
}
