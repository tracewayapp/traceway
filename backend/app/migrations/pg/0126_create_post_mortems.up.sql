CREATE TABLE IF NOT EXISTS post_mortems (
    id SERIAL PRIMARY KEY,
    organization_id INT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    incident_id INT REFERENCES check_incidents(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    tags TEXT NOT NULL DEFAULT '[]',
    created_by INT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    content_md TEXT NOT NULL DEFAULT ''
)
