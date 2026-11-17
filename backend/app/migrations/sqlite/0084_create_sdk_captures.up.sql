CREATE TABLE IF NOT EXISTS sdk_captures (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    label TEXT NOT NULL DEFAULT '',
    conditions TEXT NOT NULL DEFAULT '{}',
    actions TEXT NOT NULL DEFAULT '{}',
    expires_at DATETIME NOT NULL,
    cancelled_at DATETIME,
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS sdk_captures_project_idx ON sdk_captures (project_id, created_at);
