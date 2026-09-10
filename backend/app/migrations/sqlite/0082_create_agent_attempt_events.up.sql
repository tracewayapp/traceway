CREATE TABLE IF NOT EXISTS agent_attempt_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    attempt_id TEXT NOT NULL REFERENCES agent_attempts(id) ON DELETE CASCADE,
    seq INTEGER NOT NULL,
    kind TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS agent_attempt_events_attempt_seq_unique ON agent_attempt_events (attempt_id, seq);
