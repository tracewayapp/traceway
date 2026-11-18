CREATE TABLE IF NOT EXISTS slo_buckets (
    project_id TEXT NOT NULL,
    slo_id INTEGER NOT NULL,
    bucket_start DATETIME NOT NULL,
    good INTEGER NOT NULL DEFAULT 0,
    total INTEGER NOT NULL DEFAULT 0,
    computed_at DATETIME NOT NULL,
    UNIQUE(project_id, slo_id, bucket_start)
);
