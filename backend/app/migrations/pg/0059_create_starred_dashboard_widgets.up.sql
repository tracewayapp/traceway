CREATE TABLE IF NOT EXISTS starred_dashboard_widgets (
    id SERIAL PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    dashboard_id INT NOT NULL REFERENCES dashboards(id) ON DELETE CASCADE,
    widget_id VARCHAR(20) NOT NULL,
    position INT NOT NULL DEFAULT 0,
    col_span INT NOT NULL DEFAULT 1,
    size VARCHAR(10) NOT NULL DEFAULT 'sm',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, dashboard_id, widget_id)
)
