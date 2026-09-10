CREATE TABLE IF NOT EXISTS agent_attempts (
    id TEXT PRIMARY KEY,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    repository_id INTEGER REFERENCES repositories(id) ON DELETE SET NULL,
    profile_id INTEGER REFERENCES agent_profiles(id) ON DELETE SET NULL,
    number INTEGER NOT NULL,
    kind TEXT NOT NULL DEFAULT 'fix',
    subject_kind TEXT NOT NULL,
    subject_ref TEXT NOT NULL,
    executor TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'queued',
    resume INTEGER NOT NULL DEFAULT 0,
    base_branch TEXT NOT NULL DEFAULT '',
    fix_branch TEXT NOT NULL DEFAULT '',
    agent TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    cost_usd REAL NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    turns INTEGER NOT NULL DEFAULT 0,
    claimed_by TEXT NOT NULL DEFAULT '',
    lease_expires_at DATETIME,
    requested_by INTEGER,
    approved_by INTEGER,
    error TEXT NOT NULL DEFAULT '',
    report_key TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    started_at DATETIME,
    finished_at DATETIME,
    updated_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS agent_attempts_active_subject_unique ON agent_attempts (project_id, subject_kind, subject_ref) WHERE status IN ('pending_approval', 'queued', 'claimed', 'preparing', 'running', 'verifying', 'publishing', 'needs_input', 'awaiting_review');

CREATE INDEX IF NOT EXISTS agent_attempts_project_created_idx ON agent_attempts (project_id, created_at DESC);

CREATE INDEX IF NOT EXISTS agent_attempts_status_idx ON agent_attempts (status, created_at);
