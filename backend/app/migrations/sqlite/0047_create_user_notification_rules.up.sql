CREATE TABLE IF NOT EXISTS user_notification_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    urgency TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    delay_minutes INTEGER NOT NULL DEFAULT 0,
    contact_method_id INTEGER NOT NULL REFERENCES user_contact_methods(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL
)
