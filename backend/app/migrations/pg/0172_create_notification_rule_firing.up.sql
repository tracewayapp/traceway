CREATE TABLE IF NOT EXISTS notification_rule_firing (
    id SERIAL PRIMARY KEY,
    rule_id INT NOT NULL REFERENCES notification_rules(id) ON DELETE CASCADE,
    token TEXT NOT NULL,
    page_key VARCHAR(300) NOT NULL,
    since TIMESTAMPTZ NOT NULL,
    silent_since TIMESTAMPTZ,
    UNIQUE (rule_id, page_key)
)
