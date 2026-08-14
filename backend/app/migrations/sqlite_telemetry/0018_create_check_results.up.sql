CREATE TABLE IF NOT EXISTS check_results (
    project_id TEXT NOT NULL,
    check_id INTEGER NOT NULL DEFAULT 0,
    run_id INTEGER NOT NULL DEFAULT 0,
    check_type TEXT NOT NULL DEFAULT '',
    recorded_at DATETIME NOT NULL,
    executed_at DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT '',
    latency_ms REAL NOT NULL DEFAULT 0,
    status_code INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    executed_by TEXT NOT NULL DEFAULT '',
    screenshot_key TEXT NOT NULL DEFAULT '',
    tls_days_remaining INTEGER NOT NULL DEFAULT -1
);

CREATE INDEX IF NOT EXISTS idx_check_results_check_recorded ON check_results(project_id, check_id, recorded_at);
