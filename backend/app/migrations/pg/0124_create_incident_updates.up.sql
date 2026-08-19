CREATE TABLE IF NOT EXISTS incident_updates (
    id SERIAL PRIMARY KEY,
    incident_id INT NOT NULL REFERENCES check_incidents(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    created_by INT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL
)
