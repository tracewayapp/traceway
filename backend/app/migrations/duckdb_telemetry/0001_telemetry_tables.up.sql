CREATE TABLE IF NOT EXISTS endpoints (
    id VARCHAR NOT NULL,
    project_id VARCHAR NOT NULL,
    endpoint VARCHAR NOT NULL DEFAULT '',
    duration BIGINT NOT NULL DEFAULT 0,
    recorded_at TIMESTAMP NOT NULL,
    status_code BIGINT NOT NULL DEFAULT 0,
    body_size BIGINT NOT NULL DEFAULT 0,
    client_ip VARCHAR NOT NULL DEFAULT '',
    attributes VARCHAR NOT NULL DEFAULT '{}',
    app_version VARCHAR NOT NULL DEFAULT '',
    server_name VARCHAR NOT NULL DEFAULT '',
    distributed_trace_id VARCHAR,
    span_id VARCHAR,
    is_stream BIGINT NOT NULL DEFAULT 0,
    is_root BIGINT NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR NOT NULL,
    project_id VARCHAR NOT NULL,
    task_name VARCHAR NOT NULL DEFAULT '',
    duration BIGINT NOT NULL DEFAULT 0,
    recorded_at TIMESTAMP NOT NULL,
    client_ip VARCHAR NOT NULL DEFAULT '',
    attributes VARCHAR NOT NULL DEFAULT '{}',
    app_version VARCHAR NOT NULL DEFAULT '',
    server_name VARCHAR NOT NULL DEFAULT '',
    distributed_trace_id VARCHAR,
    span_id VARCHAR,
    is_root BIGINT NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS exception_stack_traces (
    id VARCHAR NOT NULL,
    project_id VARCHAR NOT NULL,
    trace_id VARCHAR,
    trace_type VARCHAR NOT NULL DEFAULT 'endpoint',
    exception_hash VARCHAR NOT NULL DEFAULT '',
    stack_trace VARCHAR NOT NULL DEFAULT '',
    recorded_at TIMESTAMP NOT NULL,
    attributes VARCHAR NOT NULL DEFAULT '{}',
    app_version VARCHAR NOT NULL DEFAULT '',
    server_name VARCHAR NOT NULL DEFAULT '',
    is_message BIGINT NOT NULL DEFAULT 0,
    distributed_trace_id VARCHAR,
    session_id VARCHAR
);

CREATE TABLE IF NOT EXISTS spans (
    id VARCHAR NOT NULL,
    trace_id VARCHAR NOT NULL,
    project_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL DEFAULT '',
    start_time TIMESTAMP NOT NULL,
    duration BIGINT NOT NULL DEFAULT 0,
    recorded_at TIMESTAMP NOT NULL,
    parent_span_id VARCHAR,
    attributes VARCHAR NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS metric_points (
    project_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL DEFAULT '',
    value DOUBLE NOT NULL DEFAULT 0,
    tags VARCHAR NOT NULL DEFAULT '{}',
    recorded_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS session_recordings (
    id VARCHAR NOT NULL,
    project_id VARCHAR NOT NULL,
    exception_id VARCHAR NOT NULL,
    file_path VARCHAR NOT NULL DEFAULT '',
    recorded_at TIMESTAMP NOT NULL,
    session_id VARCHAR,
    segment_index BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS archived_exceptions (
    project_id VARCHAR NOT NULL,
    exception_hash VARCHAR NOT NULL,
    archived_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE(project_id, exception_hash)
);

CREATE TABLE IF NOT EXISTS slow_endpoints (
    project_id VARCHAR NOT NULL,
    endpoint VARCHAR NOT NULL,
    offset_ms BIGINT NOT NULL DEFAULT 0,
    reason VARCHAR NOT NULL DEFAULT '',
    UNIQUE(project_id, endpoint)
);

CREATE TABLE IF NOT EXISTS fired_notifications (
    project_id VARCHAR NOT NULL,
    rule_id BIGINT NOT NULL DEFAULT 0,
    rule_type VARCHAR NOT NULL DEFAULT '',
    rule_name VARCHAR NOT NULL DEFAULT '',
    channel_type VARCHAR NOT NULL DEFAULT '',
    channel_name VARCHAR NOT NULL DEFAULT '',
    severity VARCHAR NOT NULL DEFAULT '',
    subject VARCHAR NOT NULL DEFAULT '',
    body VARCHAR NOT NULL DEFAULT '',
    status VARCHAR NOT NULL DEFAULT '',
    error_message VARCHAR NOT NULL DEFAULT '',
    endpoint VARCHAR NOT NULL DEFAULT '',
    fired_at TIMESTAMP NOT NULL,
    url VARCHAR NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS ai_traces (
    id VARCHAR NOT NULL,
    project_id VARCHAR NOT NULL,
    recorded_at TIMESTAMP NOT NULL,
    duration BIGINT NOT NULL DEFAULT 0,
    status_code BIGINT NOT NULL DEFAULT 0,
    model VARCHAR NOT NULL DEFAULT '',
    response_model VARCHAR NOT NULL DEFAULT '',
    provider VARCHAR NOT NULL DEFAULT '',
    operation VARCHAR NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    cached_tokens BIGINT NOT NULL DEFAULT 0,
    reasoning_tokens BIGINT NOT NULL DEFAULT 0,
    input_cost DOUBLE NOT NULL DEFAULT 0,
    output_cost DOUBLE NOT NULL DEFAULT 0,
    total_cost DOUBLE NOT NULL DEFAULT 0,
    trace_name VARCHAR NOT NULL DEFAULT '',
    user_id VARCHAR NOT NULL DEFAULT '',
    finish_reason VARCHAR NOT NULL DEFAULT '',
    server_name VARCHAR NOT NULL DEFAULT '',
    app_version VARCHAR NOT NULL DEFAULT '',
    storage_key VARCHAR NOT NULL DEFAULT '',
    attributes VARCHAR NOT NULL DEFAULT '{}',
    is_root BIGINT NOT NULL DEFAULT 1,
    distributed_trace_id VARCHAR
);

CREATE TABLE IF NOT EXISTS log_records (
    id VARCHAR NOT NULL,
    project_id VARCHAR NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    trace_id VARCHAR NOT NULL DEFAULT '',
    span_id VARCHAR NOT NULL DEFAULT '',
    trace_flags BIGINT NOT NULL DEFAULT 0,
    severity_text VARCHAR NOT NULL DEFAULT '',
    severity_number BIGINT NOT NULL DEFAULT 0,
    service_name VARCHAR NOT NULL DEFAULT '',
    body VARCHAR NOT NULL DEFAULT '',
    resource_schema_url VARCHAR NOT NULL DEFAULT '',
    resource_attributes VARCHAR NOT NULL DEFAULT '{}',
    scope_schema_url VARCHAR NOT NULL DEFAULT '',
    scope_name VARCHAR NOT NULL DEFAULT '',
    scope_version VARCHAR NOT NULL DEFAULT '',
    scope_attributes VARCHAR NOT NULL DEFAULT '{}',
    log_attributes VARCHAR NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR NOT NULL PRIMARY KEY,
    project_id VARCHAR NOT NULL,
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    duration BIGINT NOT NULL DEFAULT 0,
    client_ip VARCHAR NOT NULL DEFAULT '',
    attributes VARCHAR NOT NULL DEFAULT '{}',
    app_version VARCHAR NOT NULL DEFAULT '',
    server_name VARCHAR NOT NULL DEFAULT '',
    distributed_trace_id VARCHAR
);

CREATE TABLE IF NOT EXISTS profiling_stacks (
    project_id VARCHAR NOT NULL,
    service_name VARCHAR NOT NULL DEFAULT '',
    stack_hash BIGINT NOT NULL,
    stack VARCHAR NOT NULL DEFAULT '[]',
    last_seen TIMESTAMP NOT NULL,
    UNIQUE(project_id, service_name, stack_hash)
);

CREATE TABLE IF NOT EXISTS profiling_samples (
    project_id VARCHAR NOT NULL,
    profile_id VARCHAR NOT NULL,
    service_name VARCHAR NOT NULL DEFAULT '',
    type VARCHAR NOT NULL DEFAULT '',
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    stack_hash BIGINT NOT NULL,
    value BIGINT NOT NULL DEFAULT 0,
    labels VARCHAR NOT NULL DEFAULT '{}',
    server_name VARCHAR NOT NULL DEFAULT '',
    app_version VARCHAR NOT NULL DEFAULT '',
    trace_id VARCHAR NOT NULL DEFAULT '',
    span_id VARCHAR NOT NULL DEFAULT '',
    unit VARCHAR NOT NULL DEFAULT '',
    is_gauge BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS profiles (
    id VARCHAR NOT NULL,
    project_id VARCHAR NOT NULL,
    recorded_at TIMESTAMP NOT NULL,
    duration BIGINT NOT NULL DEFAULT 0,
    service_name VARCHAR NOT NULL DEFAULT '',
    profile_type VARCHAR NOT NULL DEFAULT '',
    sample_count BIGINT NOT NULL DEFAULT 0,
    total_value BIGINT NOT NULL DEFAULT 0,
    server_name VARCHAR NOT NULL DEFAULT '',
    app_version VARCHAR NOT NULL DEFAULT '',
    attributes VARCHAR NOT NULL DEFAULT '{}',
    storage_key VARCHAR NOT NULL DEFAULT '',
    trace_id VARCHAR NOT NULL DEFAULT '',
    span_id VARCHAR NOT NULL DEFAULT '',
    distributed_trace_id VARCHAR,
    unit VARCHAR NOT NULL DEFAULT '',
    is_gauge BIGINT NOT NULL DEFAULT 0
);
