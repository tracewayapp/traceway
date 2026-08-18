CREATE TABLE IF NOT EXISTS setup_tokens (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    prefix TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    organization_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_setup_tokens_user_org ON setup_tokens(user_id, organization_id);
