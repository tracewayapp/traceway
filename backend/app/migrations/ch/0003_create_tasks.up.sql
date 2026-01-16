CREATE TABLE IF NOT EXISTS tasks
(
    `id` UUID,
    `project_id` UUID,
    `task_name` LowCardinality(String),
    `duration` Int64,
    `recorded_at` DateTime,
    `client_ip` String,
    `scope` String DEFAULT '{}',
    `app_version` LowCardinality(String) DEFAULT '',
    `server_name` LowCardinality(String) DEFAULT '',
    INDEX idx_task_name task_name TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_id id TYPE bloom_filter(0.001) GRANULARITY 1
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(recorded_at)
ORDER BY (project_id, recorded_at, task_name)
SETTINGS index_granularity = 8192
