CREATE TABLE IF NOT EXISTS endpoints (
    id String,
    project_id String,
    endpoint String,
    duration Int64,
    recorded_at DateTime,
    status_code Int32,
    body_size Int32,
    client_ip String,
    scope String DEFAULT '{}',
    app_version String DEFAULT '',
    server_name String DEFAULT ''
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(recorded_at)
ORDER BY (project_id, recorded_at, endpoint);
