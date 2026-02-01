CREATE TABLE IF NOT EXISTS segments
(
    `id` UUID,
    `trace_id` UUID,
    `project_id` UUID,
    `name` LowCardinality(String),
    `start_time` DateTime64(6),
    `duration` Int64,
    `recorded_at` DateTime,
    INDEX idx_trace_id trace_id TYPE bloom_filter(0.001) GRANULARITY 1
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(recorded_at)
ORDER BY (project_id, trace_id, start_time)
SETTINGS index_granularity = 8192;
