CREATE TABLE IF NOT EXISTS synthetic_checks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    check_type TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    interval_seconds INTEGER NOT NULL DEFAULT 60,
    timeout_seconds INTEGER NOT NULL DEFAULT 30,
    config TEXT NOT NULL DEFAULT '{}',
    failure_threshold INTEGER NOT NULL DEFAULT 2,
    current_status TEXT NOT NULL DEFAULT 'unknown',
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_run_at DATETIME,
    last_state_change_at DATETIME,
    next_run_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS synthetic_checks_due_idx ON synthetic_checks (enabled, next_run_at);

CREATE INDEX IF NOT EXISTS synthetic_checks_project_idx ON synthetic_checks (project_id);

CREATE TABLE IF NOT EXISTS check_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    check_id INTEGER NOT NULL REFERENCES synthetic_checks(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL,
    check_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    scheduled_for DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    claimed_at DATETIME,
    claimed_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS check_runs_due_idx ON check_runs (status, scheduled_for);

CREATE INDEX IF NOT EXISTS check_runs_check_idx ON check_runs (check_id);

CREATE TABLE IF NOT EXISTS check_incidents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    check_id INTEGER NOT NULL REFERENCES synthetic_checks(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL,
    started_at DATETIME NOT NULL,
    resolved_at DATETIME,
    error_message TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS check_incidents_check_idx ON check_incidents (check_id, started_at);

CREATE TABLE IF NOT EXISTS synthetic_runners (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    last_seen_at DATETIME,
    version TEXT NOT NULL DEFAULT '',
    revoked_at DATETIME,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS status_pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    is_public INTEGER NOT NULL DEFAULT 0,
    check_ids TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
