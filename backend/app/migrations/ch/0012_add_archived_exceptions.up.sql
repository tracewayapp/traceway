CREATE TABLE IF NOT EXISTS archived_exceptions (
    project_id String,
    exception_hash String,
    archived_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(archived_at)
ORDER BY (project_id, exception_hash);
