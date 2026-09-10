CREATE UNIQUE INDEX IF NOT EXISTS project_telemetry_sources_unique ON project_telemetry_sources (project_id, domain, integration_id)
