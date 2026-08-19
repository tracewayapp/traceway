CREATE TABLE IF NOT EXISTS post_mortem_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_mortem_id INTEGER NOT NULL REFERENCES post_mortems(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id),
    action TEXT NOT NULL,
    changes TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS post_mortem_events_pm_idx ON post_mortem_events (post_mortem_id, created_at);
