ALTER TABLE session_recordings ADD COLUMN ended_at DATETIME DEFAULT NULL;

CREATE INDEX IF NOT EXISTS idx_session_recordings_project_recorded ON session_recordings(project_id, recorded_at);
