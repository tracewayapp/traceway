CREATE TABLE IF NOT EXISTS personal_access_tokens (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    prefix TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    last_used_at DATETIME,
    expires_at DATETIME,
    revoked INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_personal_access_tokens_user_id ON personal_access_tokens(user_id);
