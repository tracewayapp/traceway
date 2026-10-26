CREATE TABLE IF NOT EXISTS slos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sli_kind TEXT NOT NULL,
    sli_config TEXT NOT NULL DEFAULT '{}',
    target_percent REAL NOT NULL,
    window_days INTEGER NOT NULL DEFAULT 30,
    bucket_minutes INTEGER NOT NULL DEFAULT 1,
    enabled INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'no_data',
    current_sli REAL,
    budget_remaining REAL,
    burn_rate_1h REAL,
    burn_rate_6h REAL,
    computed_at DATETIME,
    history_from DATETIME,
    last_bucket_at DATETIME,
    created_by INTEGER,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS slos_project_idx ON slos (project_id);

CREATE TABLE IF NOT EXISTS notification_rule_states (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL UNIQUE REFERENCES notification_rules(id) ON DELETE CASCADE,
    firing INTEGER NOT NULL DEFAULT 0,
    since DATETIME,
    updated_at DATETIME NOT NULL
);
