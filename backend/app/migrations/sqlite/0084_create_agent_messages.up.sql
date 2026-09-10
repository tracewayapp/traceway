CREATE TABLE IF NOT EXISTS agent_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    attempt_id TEXT NOT NULL REFERENCES agent_attempts(id) ON DELETE CASCADE,
    direction TEXT NOT NULL,
    provider TEXT NOT NULL,
    link_id INTEGER REFERENCES agent_links(id) ON DELETE SET NULL,
    identity_id INTEGER REFERENCES identities(id) ON DELETE SET NULL,
    kind TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    external_ref TEXT NOT NULL DEFAULT '',
    delivered_at DATETIME,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS agent_messages_attempt_created_idx ON agent_messages (attempt_id, created_at);
