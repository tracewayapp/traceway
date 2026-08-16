CREATE TABLE IF NOT EXISTS check_incidents (
    id SERIAL PRIMARY KEY,
    check_id INT NOT NULL REFERENCES synthetic_checks(id) ON DELETE CASCADE,
    project_id UUID NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    error_message TEXT NOT NULL DEFAULT ''
)
