CREATE TABLE IF NOT EXISTS segments (
    id String,
    transaction_id String,
    project_id String,
    name String,
    start_time DateTime64(6),
    duration Int64,
    recorded_at DateTime
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(recorded_at)
ORDER BY (project_id, transaction_id, start_time)
