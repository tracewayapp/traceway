CREATE TABLE IF NOT EXISTS agent_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    agent TEXT NOT NULL,
    package TEXT NOT NULL DEFAULT '',
    package_version TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    base_url TEXT NOT NULL DEFAULT '',
    credential TEXT NOT NULL DEFAULT '',
    max_turns INTEGER NOT NULL DEFAULT 0,
    timeout_minutes INTEGER NOT NULL DEFAULT 0,
    budget_usd REAL NOT NULL DEFAULT 0,
    allowed_tools TEXT NOT NULL DEFAULT '[]',
    network_policy TEXT NOT NULL DEFAULT '{}',
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS agent_profiles_org_name_unique ON agent_profiles (organization_id, name);
