CREATE TABLE IF NOT EXISTS page_notifications (
    id SERIAL PRIMARY KEY,
    page_id INT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    level INT NOT NULL DEFAULT 0,
    iteration INT NOT NULL DEFAULT 0,
    user_id INT,
    target_desc VARCHAR(300) NOT NULL DEFAULT '',
    method_type VARCHAR(50) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_msg TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
)
