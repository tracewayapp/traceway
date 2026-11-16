CREATE TABLE IF NOT EXISTS client_metric_guard_entries (
    project_id TEXT NOT NULL,
    day INTEGER NOT NULL,
    scope TEXT NOT NULL,
    value TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    PRIMARY KEY (project_id, day, scope, value)
);

CREATE INDEX IF NOT EXISTS idx_client_metric_guard_entries_day ON client_metric_guard_entries (day, created_at);
