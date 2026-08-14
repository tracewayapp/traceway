CREATE TABLE IF NOT EXISTS check_results (
    project_id UUID,
    check_id Int64,
    run_id Int64,
    check_type String,
    recorded_at DateTime64(3),
    executed_at DateTime64(3),
    status String,
    latency_ms Float64,
    status_code Int32 DEFAULT 0,
    error_message String DEFAULT '',
    executed_by String DEFAULT '',
    screenshot_key String DEFAULT '',
    tls_days_remaining Int32 DEFAULT -1
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(recorded_at)
ORDER BY (project_id, check_id, recorded_at)
