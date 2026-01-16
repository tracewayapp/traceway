CREATE TABLE IF NOT EXISTS metric_records
(
    `project_id` UUID,
    `name` LowCardinality(String),
    `value` Float64,
    `recorded_at` DateTime,
    `server_name` LowCardinality(String) DEFAULT '',
    INDEX idx_server_name server_name TYPE bloom_filter(0.01) GRANULARITY 4
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(recorded_at)
ORDER BY (project_id, name, recorded_at)
SETTINGS index_granularity = 8192
