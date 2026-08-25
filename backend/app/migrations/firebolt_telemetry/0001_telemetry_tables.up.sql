CREATE TABLE IF NOT EXISTS endpoints (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    endpoint TEXT NOT NULL DEFAULT '',
    duration BIGINT NOT NULL DEFAULT 0,
    recorded_at TIMESTAMP NOT NULL,
    status_code BIGINT NOT NULL DEFAULT 0,
    body_size BIGINT NOT NULL DEFAULT 0,
    client_ip TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    app_version TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    distributed_trace_id TEXT,
    span_id TEXT,
    is_stream BIGINT NOT NULL DEFAULT 0,
    is_root BIGINT NOT NULL DEFAULT 1
) PRIMARY INDEX project_id, recorded_at;

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    task_name TEXT NOT NULL DEFAULT '',
    duration BIGINT NOT NULL DEFAULT 0,
    recorded_at TIMESTAMP NOT NULL,
    client_ip TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    app_version TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    distributed_trace_id TEXT,
    span_id TEXT,
    is_root BIGINT NOT NULL DEFAULT 1
) PRIMARY INDEX project_id, recorded_at;

CREATE TABLE IF NOT EXISTS exception_stack_traces (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    trace_id TEXT,
    trace_type TEXT NOT NULL DEFAULT 'endpoint',
    exception_hash TEXT NOT NULL DEFAULT '',
    stack_trace TEXT NOT NULL DEFAULT '',
    recorded_at TIMESTAMP NOT NULL,
    attributes TEXT NOT NULL DEFAULT '{}',
    app_version TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    is_message BIGINT NOT NULL DEFAULT 0,
    distributed_trace_id TEXT,
    session_id TEXT
) PRIMARY INDEX project_id, exception_hash, recorded_at;

CREATE TABLE IF NOT EXISTS spans (
    id TEXT NOT NULL,
    trace_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    start_time TIMESTAMP NOT NULL,
    duration BIGINT NOT NULL DEFAULT 0,
    recorded_at TIMESTAMP NOT NULL,
    parent_span_id TEXT,
    attributes TEXT NOT NULL DEFAULT '{}'
) PRIMARY INDEX project_id, trace_id;

CREATE TABLE IF NOT EXISTS metric_points (
    project_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    value DOUBLE PRECISION NOT NULL DEFAULT 0,
    tags TEXT NOT NULL DEFAULT '{}',
    recorded_at TIMESTAMP NOT NULL,
    server_name TEXT NOT NULL DEFAULT ''
) PRIMARY INDEX project_id, name, recorded_at;

CREATE TABLE IF NOT EXISTS session_recordings (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    exception_id TEXT NOT NULL,
    file_path TEXT NOT NULL DEFAULT '',
    recorded_at TIMESTAMP NOT NULL,
    session_id TEXT,
    segment_index BIGINT NOT NULL DEFAULT 0
) PRIMARY INDEX project_id, exception_id;

CREATE TABLE IF NOT EXISTS archived_exceptions (
    project_id TEXT NOT NULL,
    exception_hash TEXT NOT NULL,
    archived_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, exception_hash)
) PRIMARY INDEX project_id, exception_hash;

CREATE TABLE IF NOT EXISTS slow_endpoints (
    project_id TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    offset_ms BIGINT NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    UNIQUE(project_id, endpoint)
) PRIMARY INDEX project_id, endpoint;

CREATE TABLE IF NOT EXISTS fired_notifications (
    project_id TEXT NOT NULL,
    rule_id BIGINT NOT NULL DEFAULT 0,
    rule_type TEXT NOT NULL DEFAULT '',
    rule_name TEXT NOT NULL DEFAULT '',
    channel_type TEXT NOT NULL DEFAULT '',
    channel_name TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    endpoint TEXT NOT NULL DEFAULT '',
    fired_at TIMESTAMP NOT NULL,
    url TEXT NOT NULL DEFAULT ''
) PRIMARY INDEX project_id, fired_at;

CREATE TABLE IF NOT EXISTS ai_traces (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    recorded_at TIMESTAMP NOT NULL,
    duration BIGINT NOT NULL DEFAULT 0,
    status_code BIGINT NOT NULL DEFAULT 0,
    model TEXT NOT NULL DEFAULT '',
    response_model TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    operation TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    cached_tokens BIGINT NOT NULL DEFAULT 0,
    reasoning_tokens BIGINT NOT NULL DEFAULT 0,
    input_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
    output_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
    trace_name TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    finish_reason TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    app_version TEXT NOT NULL DEFAULT '',
    storage_key TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    is_root BIGINT NOT NULL DEFAULT 1,
    distributed_trace_id TEXT,
    conversation_id TEXT NOT NULL DEFAULT '',
    tool_call_count BIGINT NOT NULL DEFAULT 0,
    tool_names TEXT NOT NULL DEFAULT '',
    flagged BIGINT NOT NULL DEFAULT 0,
    flagged_terms TEXT NOT NULL DEFAULT ''
) PRIMARY INDEX project_id, recorded_at;

CREATE TABLE IF NOT EXISTS log_records (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    trace_flags BIGINT NOT NULL DEFAULT 0,
    severity_text TEXT NOT NULL DEFAULT '',
    severity_number BIGINT NOT NULL DEFAULT 0,
    service_name TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    resource_schema_url TEXT NOT NULL DEFAULT '',
    resource_attributes TEXT NOT NULL DEFAULT '{}',
    scope_schema_url TEXT NOT NULL DEFAULT '',
    scope_name TEXT NOT NULL DEFAULT '',
    scope_version TEXT NOT NULL DEFAULT '',
    scope_attributes TEXT NOT NULL DEFAULT '{}',
    log_attributes TEXT NOT NULL DEFAULT '{}'
) PRIMARY INDEX project_id, timestamp;

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    duration BIGINT NOT NULL DEFAULT 0,
    client_ip TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    app_version TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    distributed_trace_id TEXT,
    UNIQUE(id)
) PRIMARY INDEX project_id, started_at;

CREATE TABLE IF NOT EXISTS profiling_stacks (
    project_id TEXT NOT NULL,
    service_name TEXT NOT NULL DEFAULT '',
    stack_hash BIGINT NOT NULL,
    stack TEXT NOT NULL DEFAULT '[]',
    last_seen TIMESTAMP NOT NULL,
    UNIQUE(project_id, service_name, stack_hash)
) PRIMARY INDEX project_id, service_name, stack_hash;

CREATE TABLE IF NOT EXISTS profiling_samples (
    project_id TEXT NOT NULL,
    profile_id TEXT NOT NULL,
    service_name TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL DEFAULT '',
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    stack_hash BIGINT NOT NULL,
    value BIGINT NOT NULL DEFAULT 0,
    labels TEXT NOT NULL DEFAULT '{}',
    server_name TEXT NOT NULL DEFAULT '',
    app_version TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    unit TEXT NOT NULL DEFAULT '',
    is_gauge BIGINT NOT NULL DEFAULT 0
) PRIMARY INDEX project_id, service_name, start_time;

CREATE TABLE IF NOT EXISTS profiles (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    recorded_at TIMESTAMP NOT NULL,
    duration BIGINT NOT NULL DEFAULT 0,
    service_name TEXT NOT NULL DEFAULT '',
    profile_type TEXT NOT NULL DEFAULT '',
    sample_count BIGINT NOT NULL DEFAULT 0,
    total_value BIGINT NOT NULL DEFAULT 0,
    server_name TEXT NOT NULL DEFAULT '',
    app_version TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    storage_key TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    distributed_trace_id TEXT,
    unit TEXT NOT NULL DEFAULT '',
    is_gauge BIGINT NOT NULL DEFAULT 0
) PRIMARY INDEX project_id, recorded_at;

CREATE TABLE IF NOT EXISTS check_results (
    project_id TEXT NOT NULL,
    check_id BIGINT NOT NULL DEFAULT 0,
    run_id BIGINT NOT NULL DEFAULT 0,
    check_type TEXT NOT NULL DEFAULT '',
    recorded_at TIMESTAMP NOT NULL,
    executed_at TIMESTAMP NOT NULL,
    status TEXT NOT NULL DEFAULT '',
    latency_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
    status_code BIGINT NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    executed_by TEXT NOT NULL DEFAULT '',
    screenshot_key TEXT NOT NULL DEFAULT '',
    tls_days_remaining BIGINT NOT NULL DEFAULT -1,
    output_key TEXT NOT NULL DEFAULT ''
) PRIMARY INDEX project_id, check_id, recorded_at;
