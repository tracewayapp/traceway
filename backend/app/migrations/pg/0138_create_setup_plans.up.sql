CREATE TABLE IF NOT EXISTS setup_plans (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    organization_id INTEGER NOT NULL REFERENCES organizations(id),
    payload TEXT NOT NULL,
    status TEXT NOT NULL,
    reject_reason TEXT,
    result TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    decided_at TIMESTAMPTZ,
    decided_by INTEGER
)
