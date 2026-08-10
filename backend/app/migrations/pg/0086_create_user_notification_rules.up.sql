CREATE TABLE IF NOT EXISTS user_notification_rules (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    urgency VARCHAR(10) NOT NULL,
    position INT NOT NULL DEFAULT 0,
    delay_minutes INT NOT NULL DEFAULT 0,
    contact_method_id INT NOT NULL REFERENCES user_contact_methods(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
