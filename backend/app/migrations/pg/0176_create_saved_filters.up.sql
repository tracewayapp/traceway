CREATE TABLE IF NOT EXISTS saved_filters (
    id SERIAL PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    page VARCHAR(32) NOT NULL,
    name VARCHAR(100) NOT NULL,
    query TEXT NOT NULL,
    created_by INT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
)
