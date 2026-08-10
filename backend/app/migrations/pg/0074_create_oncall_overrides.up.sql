CREATE TABLE IF NOT EXISTS oncall_overrides (
    id SERIAL PRIMARY KEY,
    schedule_id INT NOT NULL REFERENCES oncall_schedules(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL,
    created_by INT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
