CREATE TABLE IF NOT EXISTS agent_links (
    id SERIAL PRIMARY KEY,
    attempt_id UUID NOT NULL REFERENCES agent_attempts(id) ON DELETE CASCADE,
    integration_id INT REFERENCES integrations(id) ON DELETE SET NULL,
    provider VARCHAR(50) NOT NULL,
    kind VARCHAR(50) NOT NULL,
    external_ref VARCHAR(500) NOT NULL,
    url VARCHAR(1000) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
