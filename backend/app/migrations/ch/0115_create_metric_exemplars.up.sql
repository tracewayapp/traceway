CREATE TABLE IF NOT EXISTS metric_exemplars (
    `project_id` UUID,
    `name` LowCardinality(String),
    `recorded_at` DateTime,
    `dims` Map(LowCardinality(String), String),
    `value` Float64,
    `session_id` String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(recorded_at)
ORDER BY (project_id, name, recorded_at)
TTL recorded_at + INTERVAL 90 DAY
SETTINGS index_granularity = 8192
