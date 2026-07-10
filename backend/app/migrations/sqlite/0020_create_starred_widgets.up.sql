CREATE TABLE IF NOT EXISTS starred_widgets (
    id INTEGER PRIMARY KEY,
    widget_id INTEGER NOT NULL UNIQUE REFERENCES widget_group_widgets(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    col_span INTEGER NOT NULL DEFAULT 1,
    size TEXT NOT NULL DEFAULT 'sm',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
