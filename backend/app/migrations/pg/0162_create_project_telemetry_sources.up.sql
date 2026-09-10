CREATE TABLE IF NOT EXISTS project_telemetry_sources (
    id SERIAL PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    domain VARCHAR(50) NOT NULL,
    integration_id INT NOT NULL REFERENCES integrations(id) ON DELETE CASCADE,
    priority INT NOT NULL DEFAULT 0
)
