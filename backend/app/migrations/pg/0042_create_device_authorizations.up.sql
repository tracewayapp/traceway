CREATE TABLE IF NOT EXISTS device_authorizations (
    device_code_hash TEXT PRIMARY KEY,
    user_code TEXT NOT NULL UNIQUE,
    client_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    user_id INTEGER REFERENCES users(id),
    interval_seconds INTEGER NOT NULL DEFAULT 5,
    last_polled_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
)
