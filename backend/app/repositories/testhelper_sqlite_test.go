//go:build !pgch

package repositories

import (
	"database/sql"
	"testing"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) {
	t.Helper()

	mainDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite (main): %v", err)
	}
	mainDB.SetMaxOpenConns(1)
	if _, err := mainDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}
	if _, err := mainDB.Exec("PRAGMA journal_mode = WAL"); err != nil {
		t.Fatalf("failed to set WAL mode: %v", err)
	}

	telemetryDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite (telemetry): %v", err)
	}
	telemetryDB.SetMaxOpenConns(1)
	if _, err := telemetryDB.Exec("PRAGMA journal_mode = WAL"); err != nil {
		t.Fatalf("failed to set WAL mode (telemetry): %v", err)
	}

	db.DB = mainDB
	db.TelemetryDB = telemetryDB
	db.Driver = lit.SQLite

	models.Init(db.Driver)

	if err := runTelemetryMigrations(telemetryDB); err != nil {
		t.Fatalf("failed to run telemetry migrations: %v", err)
	}

	t.Cleanup(func() {
		mainDB.Close()
		telemetryDB.Close()
	})
}

func runTelemetryMigrations(telemetryDB *sql.DB) error {
	ddl := `
CREATE TABLE IF NOT EXISTS endpoints (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    endpoint TEXT NOT NULL DEFAULT '',
    duration INTEGER NOT NULL DEFAULT 0,
    recorded_at DATETIME NOT NULL,
    status_code INTEGER NOT NULL DEFAULT 0,
    body_size INTEGER NOT NULL DEFAULT 0,
    client_ip TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    app_version TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    distributed_trace_id TEXT DEFAULT NULL,
    span_id TEXT DEFAULT NULL,
    is_stream INTEGER NOT NULL DEFAULT 0,
    is_root INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_endpoints_project_recorded ON endpoints(project_id, recorded_at);
CREATE INDEX IF NOT EXISTS idx_endpoints_project_endpoint ON endpoints(project_id, endpoint);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    task_name TEXT NOT NULL DEFAULT '',
    duration INTEGER NOT NULL DEFAULT 0,
    recorded_at DATETIME NOT NULL,
    client_ip TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    app_version TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    distributed_trace_id TEXT DEFAULT NULL,
    span_id TEXT DEFAULT NULL,
    is_root INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_tasks_project_recorded ON tasks(project_id, recorded_at);

CREATE TABLE IF NOT EXISTS ai_traces (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    recorded_at DATETIME NOT NULL,
    duration INTEGER NOT NULL DEFAULT 0,
    status_code INTEGER NOT NULL DEFAULT 0,
    model TEXT NOT NULL DEFAULT '',
    response_model TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    operation TEXT NOT NULL DEFAULT '',
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    input_cost REAL NOT NULL DEFAULT 0,
    output_cost REAL NOT NULL DEFAULT 0,
    total_cost REAL NOT NULL DEFAULT 0,
    trace_name TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    finish_reason TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    app_version TEXT NOT NULL DEFAULT '',
    storage_key TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    distributed_trace_id TEXT DEFAULT NULL,
    is_root INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_ai_traces_project_recorded ON ai_traces(project_id, recorded_at);

CREATE TABLE IF NOT EXISTS exception_stack_traces (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    trace_id TEXT,
    trace_type TEXT NOT NULL DEFAULT 'endpoint',
    exception_hash TEXT NOT NULL DEFAULT '',
    stack_trace TEXT NOT NULL DEFAULT '',
    recorded_at DATETIME NOT NULL,
    attributes TEXT NOT NULL DEFAULT '{}',
    app_version TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    is_message INTEGER NOT NULL DEFAULT 0,
    distributed_trace_id TEXT DEFAULT NULL,
    session_id TEXT DEFAULT NULL
);
CREATE INDEX IF NOT EXISTS idx_exceptions_project_recorded ON exception_stack_traces(project_id, recorded_at);
CREATE INDEX IF NOT EXISTS idx_exceptions_project_hash ON exception_stack_traces(project_id, exception_hash);

CREATE TABLE IF NOT EXISTS spans (
    id TEXT NOT NULL,
    trace_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    start_time DATETIME NOT NULL,
    duration INTEGER NOT NULL DEFAULT 0,
    recorded_at DATETIME NOT NULL,
    parent_span_id TEXT DEFAULT NULL,
    attributes TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_spans_project_trace ON spans(project_id, trace_id);

CREATE TABLE IF NOT EXISTS metric_points (
    project_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    value REAL NOT NULL DEFAULT 0,
    tags TEXT NOT NULL DEFAULT '{}',
    recorded_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_metric_points_project_name ON metric_points(project_id, name, recorded_at);

CREATE TABLE IF NOT EXISTS session_recordings (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    exception_id TEXT NOT NULL,
    file_path TEXT NOT NULL DEFAULT '',
    recorded_at DATETIME NOT NULL,
    session_id TEXT DEFAULT NULL,
    segment_index INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_session_recordings_project_exception ON session_recordings(project_id, exception_id);
CREATE INDEX IF NOT EXISTS idx_session_recordings_session_id ON session_recordings(session_id);

CREATE TABLE IF NOT EXISTS archived_exceptions (
    project_id TEXT NOT NULL,
    exception_hash TEXT NOT NULL,
    archived_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, exception_hash)
);

CREATE TABLE IF NOT EXISTS slow_endpoints (
    project_id TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    offset_ms INTEGER NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    UNIQUE(project_id, endpoint)
);

CREATE TABLE IF NOT EXISTS fired_notifications (
    project_id TEXT NOT NULL,
    rule_id INTEGER NOT NULL DEFAULT 0,
    rule_type TEXT NOT NULL DEFAULT '',
    rule_name TEXT NOT NULL DEFAULT '',
    channel_type TEXT NOT NULL DEFAULT '',
    channel_name TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    endpoint TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    fired_at DATETIME NOT NULL
);
`

	statements := splitStatements(ddl)
	for _, stmt := range statements {
		if _, err := telemetryDB.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func splitStatements(sql string) []string {
	var stmts []string
	current := ""
	for _, line := range splitLines(sql) {
		trimmed := trimSpace(line)
		if trimmed == "" {
			continue
		}
		current += line + "\n"
		if len(trimmed) > 0 && trimmed[len(trimmed)-1] == ';' {
			stmts = append(stmts, current)
			current = ""
		}
	}
	if trimSpace(current) != "" {
		stmts = append(stmts, current)
	}
	return stmts
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}
