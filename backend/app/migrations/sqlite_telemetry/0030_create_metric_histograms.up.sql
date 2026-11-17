CREATE TABLE IF NOT EXISTS metric_histograms (
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    recorded_at DATETIME NOT NULL,
    grain_seconds INTEGER NOT NULL DEFAULT 60,
    dims TEXT NOT NULL DEFAULT '{}',
    cnt REAL NOT NULL DEFAULT 0,
    total REAL NOT NULL DEFAULT 0,
    min_val REAL NOT NULL DEFAULT 0,
    max_val REAL NOT NULL DEFAULT 0,
    zero_cnt REAL NOT NULL DEFAULT 0,
    buckets TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_metric_histograms_lookup ON metric_histograms(project_id, name, recorded_at);
CREATE INDEX IF NOT EXISTS idx_metric_histograms_grain ON metric_histograms(grain_seconds, recorded_at);
CREATE TABLE IF NOT EXISTS metric_exemplars (
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    recorded_at DATETIME NOT NULL,
    dims TEXT NOT NULL DEFAULT '{}',
    value REAL NOT NULL,
    session_id TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_metric_exemplars_lookup ON metric_exemplars(project_id, name, recorded_at);
