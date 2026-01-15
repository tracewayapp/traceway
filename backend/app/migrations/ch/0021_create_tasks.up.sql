CREATE TABLE IF NOT EXISTS tasks (
    id String,
    project_id String,
    task_name String,
    duration Int64,
    recorded_at DateTime,
    client_ip String,
    scope String DEFAULT '{}',
    app_version String DEFAULT '',
    server_name String DEFAULT ''
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(recorded_at)
ORDER BY (project_id, recorded_at, task_name);
