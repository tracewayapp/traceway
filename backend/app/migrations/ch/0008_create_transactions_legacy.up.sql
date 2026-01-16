CREATE TABLE IF NOT EXISTS transactions
(
    `id` String,
    `endpoint` String,
    `duration` Int64,
    `recorded_at` DateTime,
    `status_code` Int32,
    `body_size` Int32,
    `client_ip` String,
    `project_id` String DEFAULT '00000000-0000-0000-0000-000000000001',
    `scope` String DEFAULT '{}',
    `app_version` String DEFAULT '',
    `server_name` String DEFAULT ''
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(recorded_at)
ORDER BY (recorded_at, endpoint)
SETTINGS index_granularity = 8192
