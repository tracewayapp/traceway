CREATE INDEX IF NOT EXISTS github_issues_open_idx ON github_issues (project_id, issue_key) WHERE closed_at IS NULL
