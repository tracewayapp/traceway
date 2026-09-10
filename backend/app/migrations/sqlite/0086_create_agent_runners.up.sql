CREATE TABLE IF NOT EXISTS agent_runners (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    version TEXT NOT NULL DEFAULT '',
    capabilities TEXT NOT NULL DEFAULT '{}',
    first_seen_at DATETIME NOT NULL,
    last_seen_at DATETIME NOT NULL
);
