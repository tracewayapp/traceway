CREATE TABLE IF NOT EXISTS notification_rule_firing (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL REFERENCES notification_rules(id) ON DELETE CASCADE,
    token TEXT NOT NULL,
    since DATETIME NOT NULL,
    silent_since DATETIME,
    UNIQUE (rule_id, token)
);
