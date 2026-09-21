package migrations

import (
	"database/sql"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestTraceIdentityFreshInstall(t *testing.T) {
	target, source, _, _, run := traceIdentityDatabase(t)
	if err := run(source); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"sessions", "endpoints_v2", "tasks_v2", "ai_traces_v2", "exceptions_v2", "profiles"} {
		assertCleanTraceColumns(t, target, table)
	}
}

func TestTraceIdentityUpgrade(t *testing.T) {
	target, source, dir, firstCleanup, run := traceIdentityDatabase(t)
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(source, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() < firstCleanup && strings.HasSuffix(entry.Name(), ".up.sql") {
			path := dir + "/" + entry.Name()
			data, err := fs.ReadFile(source, path)
			if err != nil {
				t.Fatal(err)
			}
			before[path] = &fstest.MapFile{Data: data}
		}
	}
	if err := run(before); err != nil {
		t.Fatal(err)
	}
	exec := func(query string) {
		t.Helper()
		if _, err := target.Exec(query); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	const project = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	const trace = "0102030405060708090a0b0c0d0e0f10"
	const span = "0102030405060708"
	for i, old := range []string{"'01020304-0506-0708-090A-0B0C0D0E0F10'", "NULL", "'00000000-0000-0000-0000-000000000000'"} {
		exec(fmt.Sprintf("INSERT INTO sessions (id, project_id, started_at, distributed_trace_id) VALUES ('00000000-0000-0000-0000-%012d', '%s', '2026-09-21 12:00:00', %s)", i+1, project, old))
	}
	for _, table := range []string{"endpoints_v2", "tasks_v2", "ai_traces_v2", "exceptions_v2", "profiles"} {
		legacy := "linked_trace_id"
		if table == "profiles" {
			legacy = "distributed_trace_id"
		}
		exec(fmt.Sprintf("INSERT INTO %s (id, project_id, recorded_at, trace_id, span_id, %s) VALUES ('11111111-1111-1111-1111-111111111111', '%s', '2026-09-21 12:00:00', '%s', '%s', '99999999-9999-9999-9999-999999999999')", table, legacy, project, trace, span))
	}
	if err := run(source); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{trace, "", ""} {
		var got string
		if err := target.QueryRow(fmt.Sprintf("SELECT trace_id FROM sessions WHERE id = '00000000-0000-0000-0000-%012d'", i+1)).Scan(&got); err != nil || got != want {
			t.Fatalf("session %d: trace_id = %q, want %q: %v", i, got, want, err)
		}
	}
	for _, table := range []string{"endpoints_v2", "tasks_v2", "ai_traces_v2", "exceptions_v2", "profiles"} {
		var gotTrace, gotSpan string
		if err := target.QueryRow("SELECT trace_id, span_id FROM "+table).Scan(&gotTrace, &gotSpan); err != nil || gotTrace != trace || gotSpan != span {
			t.Fatalf("%s lost its actual trace/span identity: %q/%q: %v", table, gotTrace, gotSpan, err)
		}
	}
	for _, table := range []string{"sessions", "endpoints_v2", "tasks_v2", "ai_traces_v2", "exceptions_v2", "profiles"} {
		assertCleanTraceColumns(t, target, table)
	}
	// A restart must not repeat a destructive migration or erase the converted IDs.
	if err := run(source); err != nil {
		t.Fatal(err)
	}
}

func assertCleanTraceColumns(t *testing.T, target *sql.DB, table string) {
	t.Helper()
	rows, err := target.Query("SELECT * FROM " + table + " LIMIT 0")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, column := range columns {
		if column == "linked_trace_id" || column == "distributed_trace_id" {
			t.Errorf("%s retains obsolete column %s", table, column)
		}
		found = found || column == "trace_id"
	}
	if !found {
		t.Errorf("%s is missing trace_id", table)
	}
}
