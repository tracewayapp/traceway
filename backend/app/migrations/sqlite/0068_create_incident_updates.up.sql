CREATE TABLE IF NOT EXISTS incident_updates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    incident_id INTEGER NOT NULL REFERENCES check_incidents(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    created_by INTEGER REFERENCES users(id),
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS incident_updates_incident_idx ON incident_updates (incident_id, created_at);
