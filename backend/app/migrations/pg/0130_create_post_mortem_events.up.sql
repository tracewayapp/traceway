CREATE TABLE IF NOT EXISTS post_mortem_events (
    id SERIAL PRIMARY KEY,
    post_mortem_id INT NOT NULL REFERENCES post_mortems(id) ON DELETE CASCADE,
    user_id INT REFERENCES users(id),
    action TEXT NOT NULL,
    changes TEXT NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL
)
