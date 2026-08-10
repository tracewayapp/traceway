CREATE TABLE IF NOT EXISTS page_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    page_id INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    level INTEGER NOT NULL DEFAULT 0,
    iteration INTEGER NOT NULL DEFAULT 0,
    user_id INTEGER,
    target_desc TEXT NOT NULL DEFAULT '',
    method_type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    error_msg TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    sent_at DATETIME
);
