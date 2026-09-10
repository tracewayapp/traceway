CREATE TABLE IF NOT EXISTS project_telemetry_sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    integration_id INTEGER NOT NULL REFERENCES integrations(id) ON DELETE CASCADE,
    priority INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS project_telemetry_sources_unique ON project_telemetry_sources (project_id, domain, integration_id);
