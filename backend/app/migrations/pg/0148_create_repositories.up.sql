CREATE TABLE IF NOT EXISTS repositories (
    id SERIAL PRIMARY KEY,
    project_id UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    integration_id INT REFERENCES integrations(id) ON DELETE SET NULL,
    owner VARCHAR(200) NOT NULL,
    name VARCHAR(200) NOT NULL,
    default_branch VARCHAR(200) NOT NULL DEFAULT 'main',
    image VARCHAR(500) NOT NULL DEFAULT '',
    setup_command TEXT NOT NULL DEFAULT '',
    test_command TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
