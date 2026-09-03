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
// history. Both are unbounded and invisible to functional tests.
//
// The first loop guards the indexes behind known reads so a later migration
// cannot narrow or drop one unnoticed. The second inverts the question: every
// index that leads with project_id must reach its table's time column, so a
// short index fails here the day it lands rather than when somebody remembers
// to list it. Indexes that stop short on purpose are named with the reason,
// and each entry is itself checked so it cannot outlive the index it excuses.
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
		{"ai_traces", "conversation_id"},
		{"exception_stack_traces", "exception_hash"},
		{"exception_stack_traces", "session_id"},
		{"check_results", "check_id"},
		{"log_records", "trace_id"},
		{"log_records", "service_name"},
		{"metric_points", "name"},
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

	intentionallyShort := map[string]string{
		"idx_spans_project_trace":                  "a trace has tens of spans; after the trace seek, the recorded_at window and the start_time sort touch only those rows",
		"idx_session_recordings_project_exception": "an exception has at most a handful of recordings, so ordering them costs nothing",
	}
	for _, ix := range indexes {
		timeCol, pruned := timeColumn[ix.table]
		if ix.cols[0] != "project_id" || !pruned {
			continue
		}
		_, listed := intentionallyShort[ix.name]
		delete(intentionallyShort, ix.name)
		reaches := slices.Contains(ix.cols[1:], timeCol)
		shape := ix.table + "(" + strings.Join(ix.cols, ", ") + ")"
		switch {
		case reaches && listed:
			t.Errorf("%s on %s reaches %s now; remove it from intentionallyShort", ix.name, shape, timeCol)
		case !reaches && !listed:
			t.Errorf("%s on %s leads with project_id but never reaches %s; widen it, or add it to intentionallyShort with the reason", ix.name, shape, timeCol)
		}
	}
	for index := range intentionallyShort {
		t.Errorf("intentionallyShort names %s, which is no longer an index leading with project_id on a pruned table; remove the entry", index)
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
