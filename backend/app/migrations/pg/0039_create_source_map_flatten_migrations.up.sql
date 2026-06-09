CREATE TABLE IF NOT EXISTS source_map_flatten_migrations (
    id SERIAL PRIMARY KEY,
    project_id UUID NOT NULL UNIQUE,
    migrated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
)
