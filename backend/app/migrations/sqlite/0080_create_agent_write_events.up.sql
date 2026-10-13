CREATE TABLE IF NOT EXISTS agent_write_events (
    id TEXT PRIMARY KEY,
    organization_id INTEGER NOT NULL,
    project_id TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    credential_kind TEXT NOT NULL,
    credential_ref TEXT NOT NULL,
    method TEXT NOT NULL,
    route TEXT NOT NULL,
    path TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_write_events_org_created ON agent_write_events(organization_id, created_at);
