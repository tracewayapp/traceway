CREATE TABLE IF NOT EXISTS synthetic_runners (
    id SERIAL PRIMARY KEY,
    organization_id INT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    last_seen_at TIMESTAMPTZ,
    version VARCHAR(50) NOT NULL DEFAULT '',
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
