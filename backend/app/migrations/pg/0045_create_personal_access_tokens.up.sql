CREATE TABLE IF NOT EXISTS personal_access_tokens (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    prefix TEXT NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL
)
