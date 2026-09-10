CREATE TABLE IF NOT EXISTS agent_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    attempt_id TEXT NOT NULL REFERENCES agent_attempts(id) ON DELETE CASCADE,
    integration_id INTEGER REFERENCES integrations(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    kind TEXT NOT NULL,
    external_ref TEXT NOT NULL,
    url TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS agent_links_provider_kind_ref_unique ON agent_links (provider, kind, external_ref);

CREATE INDEX IF NOT EXISTS agent_links_attempt_idx ON agent_links (attempt_id);
