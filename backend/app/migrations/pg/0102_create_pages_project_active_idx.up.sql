CREATE INDEX IF NOT EXISTS pages_project_active_idx ON pages (project_id, created_at DESC) WHERE status <> 'resolved'
