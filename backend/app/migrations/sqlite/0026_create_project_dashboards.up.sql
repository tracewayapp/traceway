CREATE TABLE IF NOT EXISTS project_dashboards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    dashboard_id INTEGER NOT NULL REFERENCES dashboards(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    UNIQUE(project_id, dashboard_id)
);

CREATE INDEX IF NOT EXISTS idx_project_dashboards_dashboard_id ON project_dashboards(dashboard_id);
