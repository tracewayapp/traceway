CREATE TABLE IF NOT EXISTS identities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    display TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS identities_provider_external_unique ON identities (provider, external_id);

CREATE INDEX IF NOT EXISTS identities_user_idx ON identities (user_id);
