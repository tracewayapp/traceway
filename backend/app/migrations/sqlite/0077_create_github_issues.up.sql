CREATE TABLE IF NOT EXISTS github_issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL,
    channel_id INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    issue_key TEXT NOT NULL DEFAULT '',
    owner TEXT NOT NULL DEFAULT '',
    repo TEXT NOT NULL DEFAULT '',
    issue_number INTEGER NOT NULL,
    created_at DATETIME NOT NULL,
    closed_at DATETIME
);

CREATE INDEX IF NOT EXISTS github_issues_open_idx ON github_issues (project_id, issue_key) WHERE closed_at IS NULL;
