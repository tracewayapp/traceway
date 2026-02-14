CREATE TABLE IF NOT EXISTS session_recordings
(
    `id` UUID,
    `project_id` UUID,
    `exception_id` UUID,
    `events` String,
    `recorded_at` DateTime,
    INDEX idx_exception_id exception_id TYPE bloom_filter(0.001) GRANULARITY 1
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(recorded_at)
ORDER BY (project_id, recorded_at, exception_id)
SETTINGS index_granularity = 8192
