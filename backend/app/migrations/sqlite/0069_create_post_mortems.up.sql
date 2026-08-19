CREATE TABLE IF NOT EXISTS post_mortems (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    incident_id INTEGER REFERENCES check_incidents(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    tags TEXT NOT NULL DEFAULT '[]',
    created_by INTEGER REFERENCES users(id),
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    content_md TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS post_mortems_org_idx ON post_mortems (organization_id, updated_at);

CREATE UNIQUE INDEX IF NOT EXISTS post_mortems_incident_unique ON post_mortems (incident_id) WHERE incident_id IS NOT NULL;
