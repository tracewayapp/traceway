CREATE TABLE IF NOT EXISTS check_runs (
    id SERIAL PRIMARY KEY,
    check_id INT NOT NULL REFERENCES synthetic_checks(id) ON DELETE CASCADE,
    project_id UUID NOT NULL,
    check_type VARCHAR(20) NOT NULL,
    status VARCHAR(10) NOT NULL DEFAULT 'queued',
    scheduled_for TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    claimed_at TIMESTAMPTZ,
    claimed_by VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
