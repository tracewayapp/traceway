ALTER TABLE check_incidents RENAME TO check_incidents_old;

CREATE TABLE check_incidents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    check_id INTEGER REFERENCES synthetic_checks(id) ON DELETE CASCADE,
    project_id TEXT,
    status_page_id INTEGER REFERENCES status_pages(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    started_at DATETIME NOT NULL,
    resolved_at DATETIME,
    error_message TEXT NOT NULL DEFAULT ''
);

INSERT INTO check_incidents (id, check_id, project_id, started_at, resolved_at, error_message)
SELECT id, check_id, project_id, started_at, resolved_at, error_message FROM check_incidents_old;

DROP TABLE check_incidents_old;

CREATE INDEX IF NOT EXISTS check_incidents_check_idx ON check_incidents (check_id, started_at);

CREATE INDEX IF NOT EXISTS check_incidents_status_page_idx ON check_incidents (status_page_id, started_at);
