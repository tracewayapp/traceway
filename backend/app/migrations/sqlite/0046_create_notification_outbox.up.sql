CREATE TABLE IF NOT EXISTS notification_outbox (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    adapter_type TEXT NOT NULL,
    adapter_config TEXT NOT NULL DEFAULT '{}',
    message TEXT NOT NULL DEFAULT '{}',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at DATETIME NOT NULL,
    claimed_at DATETIME,
    cancel_key TEXT NOT NULL DEFAULT '',
    page_notification_id INTEGER,
    rule_id INTEGER,
    project_id TEXT,
    channel_name TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    sent_at DATETIME
);

CREATE INDEX IF NOT EXISTS notification_outbox_due_idx ON notification_outbox (status, next_attempt_at);

CREATE INDEX IF NOT EXISTS notification_outbox_cancel_idx ON notification_outbox (cancel_key) WHERE cancel_key <> '';
