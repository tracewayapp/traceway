CREATE TABLE IF NOT EXISTS endpoints
(
    `id` UUID,
    `project_id` UUID,
    `endpoint` LowCardinality(String),
    `duration` Int64,
    `recorded_at` DateTime,
    `status_code` Int16,
    `body_size` Int32,
    `client_ip` String,
    `scope` String DEFAULT '{}',
    `app_version` LowCardinality(String) DEFAULT '',
    `server_name` LowCardinality(String) DEFAULT '',
    INDEX idx_endpoint endpoint TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_status_code status_code TYPE set(100) GRANULARITY 4,
    INDEX idx_id id TYPE bloom_filter(0.001) GRANULARITY 1
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(recorded_at)
ORDER BY (project_id, recorded_at, endpoint)
SETTINGS index_granularity = 8192
