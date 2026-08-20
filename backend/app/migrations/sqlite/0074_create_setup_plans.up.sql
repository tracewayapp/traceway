CREATE TABLE IF NOT EXISTS setup_plans (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    organization_id INTEGER NOT NULL,
    payload TEXT NOT NULL,
    status TEXT NOT NULL,
    reject_reason TEXT,
    result TEXT,
    created_at DATETIME NOT NULL,
    decided_at DATETIME,
    decided_by INTEGER
);

CREATE INDEX IF NOT EXISTS idx_setup_plans_user_org ON setup_plans(user_id, organization_id);

CREATE INDEX IF NOT EXISTS idx_setup_plans_org ON setup_plans(organization_id);
