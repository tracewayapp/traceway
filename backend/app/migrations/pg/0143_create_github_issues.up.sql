CREATE TABLE IF NOT EXISTS github_issues (
    id SERIAL PRIMARY KEY,
    project_id UUID NOT NULL,
    channel_id INT NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    issue_key VARCHAR(200) NOT NULL DEFAULT '',
    owner VARCHAR(200) NOT NULL DEFAULT '',
    repo VARCHAR(200) NOT NULL DEFAULT '',
    issue_number INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ
)
