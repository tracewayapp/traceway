CREATE TABLE IF NOT EXISTS agent_messages (
    id SERIAL PRIMARY KEY,
    attempt_id UUID NOT NULL REFERENCES agent_attempts(id) ON DELETE CASCADE,
    direction VARCHAR(10) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    link_id INT REFERENCES agent_links(id) ON DELETE SET NULL,
    identity_id INT REFERENCES identities(id) ON DELETE SET NULL,
    kind VARCHAR(50) NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    external_ref VARCHAR(500) NOT NULL DEFAULT '',
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
