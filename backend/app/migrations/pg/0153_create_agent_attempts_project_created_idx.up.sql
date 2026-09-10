CREATE INDEX IF NOT EXISTS agent_attempts_project_created_idx ON agent_attempts (project_id, created_at DESC)
