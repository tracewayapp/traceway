CREATE TABLE IF NOT EXISTS pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    policy_id INTEGER REFERENCES escalation_policies(id) ON DELETE SET NULL,
    policy_snapshot TEXT NOT NULL DEFAULT '{}',
    rule_id INTEGER,
    rule_name TEXT NOT NULL DEFAULT '',
    rule_type TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    dedup_key TEXT NOT NULL DEFAULT '',
    event_count INTEGER NOT NULL DEFAULT 1,
    last_event_at DATETIME NOT NULL,
    escalation_level INTEGER NOT NULL DEFAULT -1,
    repeat_iteration INTEGER NOT NULL DEFAULT 0,
    next_escalation_at DATETIME,
    last_escalated_at DATETIME,
    acknowledged_by INTEGER,
    acknowledged_at DATETIME,
    resolved_by INTEGER,
    resolved_at DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS pages_dedup_open_unique ON pages (dedup_key) WHERE status <> 'resolved';

CREATE INDEX IF NOT EXISTS pages_due_idx ON pages (status, next_escalation_at);
