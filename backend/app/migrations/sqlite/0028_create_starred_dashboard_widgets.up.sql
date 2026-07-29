CREATE TABLE IF NOT EXISTS starred_dashboard_widgets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    dashboard_id INTEGER NOT NULL REFERENCES dashboards(id) ON DELETE CASCADE,
    widget_id TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    col_span INTEGER NOT NULL DEFAULT 1,
    size TEXT NOT NULL DEFAULT 'sm',
    created_at DATETIME NOT NULL,
    UNIQUE(project_id, dashboard_id, widget_id)
);
