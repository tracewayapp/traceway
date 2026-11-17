CREATE TABLE IF NOT EXISTS client_metric_guard_entries (
    project_id TEXT NOT NULL,
    day BIGINT NOT NULL,
    scope TEXT NOT NULL,
    value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (project_id, day, scope, value)
)
