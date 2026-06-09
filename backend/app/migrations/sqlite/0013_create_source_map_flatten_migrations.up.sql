CREATE TABLE IF NOT EXISTS source_map_flatten_migrations (
    id INTEGER PRIMARY KEY,
    project_id TEXT NOT NULL UNIQUE,
    migrated_at DATETIME NOT NULL DEFAULT (datetime('now'))
)
