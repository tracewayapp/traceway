CREATE TABLE IF NOT EXISTS check_results (
    project_id VARCHAR NOT NULL,
    check_id BIGINT NOT NULL DEFAULT 0,
    run_id BIGINT NOT NULL DEFAULT 0,
    check_type VARCHAR NOT NULL DEFAULT '',
    recorded_at TIMESTAMP NOT NULL,
    executed_at TIMESTAMP NOT NULL,
    status VARCHAR NOT NULL DEFAULT '',
    latency_ms DOUBLE NOT NULL DEFAULT 0,
    status_code BIGINT NOT NULL DEFAULT 0,
    error_message VARCHAR NOT NULL DEFAULT '',
    executed_by VARCHAR NOT NULL DEFAULT '',
    screenshot_key VARCHAR NOT NULL DEFAULT '',
    tls_days_remaining BIGINT NOT NULL DEFAULT -1
);
