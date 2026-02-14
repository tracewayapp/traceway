CREATE TABLE IF NOT EXISTS source_maps (
    id SERIAL PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id),
    version VARCHAR(255) NOT NULL,
    file_name VARCHAR(500) NOT NULL,
    storage_key VARCHAR(1000) NOT NULL,
    file_size BIGINT NOT NULL,
    uploaded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, version, file_name)
)
