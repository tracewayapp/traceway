CREATE TABLE IF NOT EXISTS notification_outbox (
    id SERIAL PRIMARY KEY,
    kind VARCHAR(30) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    adapter_type VARCHAR(50) NOT NULL,
    adapter_config TEXT NOT NULL DEFAULT '{}',
    message TEXT NOT NULL DEFAULT '{}',
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    claimed_at TIMESTAMPTZ,
    cancel_key VARCHAR(100) NOT NULL DEFAULT '',
    page_notification_id INT,
    rule_id INT,
    project_id UUID,
    channel_name VARCHAR(200) NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
)
