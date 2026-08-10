CREATE TABLE IF NOT EXISTS escalation_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    definition TEXT NOT NULL DEFAULT '{}',
    created_by INTEGER REFERENCES users(id),
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS escalation_policies_org_name_unique ON escalation_policies (organization_id, LOWER(name));
