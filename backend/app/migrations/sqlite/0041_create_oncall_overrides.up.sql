CREATE TABLE IF NOT EXISTS oncall_overrides (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    schedule_id INTEGER NOT NULL REFERENCES oncall_schedules(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    start_at DATETIME NOT NULL,
    end_at DATETIME NOT NULL,
    created_by INTEGER REFERENCES users(id),
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS oncall_overrides_schedule_end_idx ON oncall_overrides (schedule_id, end_at);
