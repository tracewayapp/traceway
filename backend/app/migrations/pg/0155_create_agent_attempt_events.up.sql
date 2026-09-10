CREATE TABLE IF NOT EXISTS agent_attempt_events (
    id BIGSERIAL PRIMARY KEY,
    attempt_id UUID NOT NULL REFERENCES agent_attempts(id) ON DELETE CASCADE,
    seq INT NOT NULL,
    kind VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
