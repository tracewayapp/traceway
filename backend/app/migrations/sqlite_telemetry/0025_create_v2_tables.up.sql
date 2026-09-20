CREATE TABLE IF NOT EXISTS spans_v2 (
    project_id TEXT NOT NULL,
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    parent_span_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    span_kind INTEGER NOT NULL DEFAULT 0,
    status_code INTEGER NOT NULL DEFAULT 0,
    status_message TEXT NOT NULL DEFAULT '',
    service_name TEXT NOT NULL DEFAULT '',
    scope_name TEXT NOT NULL DEFAULT '',
    scope_version TEXT NOT NULL DEFAULT '',
    start_time_unix_nano TEXT NOT NULL DEFAULT '0',
    end_time_unix_nano TEXT NOT NULL DEFAULT '0',
    duration INTEGER NOT NULL DEFAULT 0,
    recorded_at DATETIME NOT NULL,
    trace_state TEXT NOT NULL DEFAULT '',
    flags INTEGER NOT NULL DEFAULT 0,
    dropped_attributes_count INTEGER NOT NULL DEFAULT 0,
    dropped_events_count INTEGER NOT NULL DEFAULT 0,
    dropped_links_count INTEGER NOT NULL DEFAULT 0,
    resource_schema_url TEXT NOT NULL DEFAULT '',
    scope_schema_url TEXT NOT NULL DEFAULT '',
    span_attributes TEXT NOT NULL DEFAULT '{}',
    resource TEXT NOT NULL DEFAULT '{}',
    scope TEXT NOT NULL DEFAULT '{}',
    events TEXT NOT NULL DEFAULT '[]',
    links TEXT NOT NULL DEFAULT '[]',
    otlp BLOB
);

CREATE INDEX IF NOT EXISTS idx_spans_v2_project_id_trace_id_recorded_at ON spans_v2(project_id, trace_id, recorded_at);

CREATE INDEX IF NOT EXISTS idx_spans_v2_project_id_recorded_at ON spans_v2(project_id, recorded_at);

CREATE TABLE IF NOT EXISTS endpoints_v2 (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    parent_span_id TEXT NOT NULL DEFAULT '',
    linked_trace_id TEXT NOT NULL DEFAULT '',
    endpoint TEXT NOT NULL DEFAULT '',
    duration INTEGER NOT NULL DEFAULT 0,
    recorded_at DATETIME NOT NULL,
    status_code INTEGER NOT NULL DEFAULT 0,
    body_size INTEGER NOT NULL DEFAULT 0,
    client_ip TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    app_version TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    is_stream INTEGER NOT NULL DEFAULT 0,
    is_root INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_endpoints_v2_project_id_endpoint_recorded_at ON endpoints_v2(project_id, endpoint, recorded_at);

CREATE INDEX IF NOT EXISTS idx_endpoints_v2_project_id_recorded_at ON endpoints_v2(project_id, recorded_at);

CREATE INDEX IF NOT EXISTS idx_endpoints_v2_trace_id ON endpoints_v2(trace_id);

CREATE INDEX IF NOT EXISTS idx_endpoints_v2_linked_trace_id ON endpoints_v2(linked_trace_id) WHERE linked_trace_id != '';

CREATE TABLE IF NOT EXISTS tasks_v2 (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    parent_span_id TEXT NOT NULL DEFAULT '',
    linked_trace_id TEXT NOT NULL DEFAULT '',
    task_name TEXT NOT NULL DEFAULT '',
    duration INTEGER NOT NULL DEFAULT 0,
    recorded_at DATETIME NOT NULL,
    client_ip TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    app_version TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    is_root INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_tasks_v2_project_id_task_name_recorded_at ON tasks_v2(project_id, task_name, recorded_at);

CREATE INDEX IF NOT EXISTS idx_tasks_v2_project_id_recorded_at ON tasks_v2(project_id, recorded_at);

CREATE INDEX IF NOT EXISTS idx_tasks_v2_trace_id ON tasks_v2(trace_id);

CREATE INDEX IF NOT EXISTS idx_tasks_v2_linked_trace_id ON tasks_v2(linked_trace_id) WHERE linked_trace_id != '';

CREATE TABLE IF NOT EXISTS ai_traces_v2 (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    parent_span_id TEXT NOT NULL DEFAULT '',
    linked_trace_id TEXT NOT NULL DEFAULT '',
    recorded_at DATETIME NOT NULL,
    duration INTEGER NOT NULL DEFAULT 0,
    status_code INTEGER NOT NULL DEFAULT 0,
    model TEXT NOT NULL DEFAULT '',
    response_model TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    operation TEXT NOT NULL DEFAULT '',
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    input_cost REAL NOT NULL DEFAULT 0,
    output_cost REAL NOT NULL DEFAULT 0,
    total_cost REAL NOT NULL DEFAULT 0,
    trace_name TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    finish_reason TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    app_version TEXT NOT NULL DEFAULT '',
    storage_key TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    is_root INTEGER NOT NULL DEFAULT 1,
    conversation_id TEXT NOT NULL DEFAULT '',
    tool_call_count INTEGER NOT NULL DEFAULT 0,
    tool_names TEXT NOT NULL DEFAULT '',
    flagged INTEGER NOT NULL DEFAULT 0,
    flagged_terms TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_ai_traces_v2_project_id_conversation_id_recorded_at ON ai_traces_v2(project_id, conversation_id, recorded_at);

CREATE INDEX IF NOT EXISTS idx_ai_traces_v2_project_id_trace_name_recorded_at ON ai_traces_v2(project_id, trace_name, recorded_at);

CREATE INDEX IF NOT EXISTS idx_ai_traces_v2_project_id_recorded_at ON ai_traces_v2(project_id, recorded_at);

CREATE INDEX IF NOT EXISTS idx_ai_traces_v2_trace_id ON ai_traces_v2(trace_id);

CREATE INDEX IF NOT EXISTS idx_ai_traces_v2_linked_trace_id ON ai_traces_v2(linked_trace_id) WHERE linked_trace_id != '';

CREATE TABLE IF NOT EXISTS exceptions_v2 (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    trace_type TEXT NOT NULL DEFAULT '',
    exception_hash TEXT NOT NULL DEFAULT '',
    stack_trace TEXT NOT NULL DEFAULT '',
    recorded_at DATETIME NOT NULL,
    attributes TEXT NOT NULL DEFAULT '{}',
    app_version TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    is_message INTEGER NOT NULL DEFAULT 0,
    linked_trace_id TEXT NOT NULL DEFAULT '',
    session_id TEXT DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_exceptions_v2_session_id_recorded_at ON exceptions_v2(session_id, recorded_at);

CREATE INDEX IF NOT EXISTS idx_exceptions_v2_project_id_exception_hash_recorded_at ON exceptions_v2(project_id, exception_hash, recorded_at);

CREATE INDEX IF NOT EXISTS idx_exceptions_v2_project_id_recorded_at ON exceptions_v2(project_id, recorded_at);

CREATE INDEX IF NOT EXISTS idx_exceptions_v2_trace_id ON exceptions_v2(trace_id);

CREATE INDEX IF NOT EXISTS idx_exceptions_v2_linked_trace_id ON exceptions_v2(linked_trace_id) WHERE linked_trace_id != '';
