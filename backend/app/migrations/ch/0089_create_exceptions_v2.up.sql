CREATE TABLE IF NOT EXISTS exceptions_v2 (
    id UUID,
    project_id UUID,
    trace_id String DEFAULT '',
    span_id String DEFAULT '',
    trace_type LowCardinality(String) DEFAULT '',
    exception_hash String,
    stack_trace String,
    recorded_at DateTime64(6),
    attributes String DEFAULT '{}',
    app_version LowCardinality(String) DEFAULT '',
    server_name LowCardinality(String) DEFAULT '',
    is_message UInt8 DEFAULT 0,
    linked_trace_id String DEFAULT '',
    session_id Nullable(UUID) DEFAULT NULL,
    INDEX idx_exceptions_v2_exception_hash exception_hash TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_exceptions_v2_is_message is_message TYPE set(2) GRANULARITY 1,
    INDEX idx_exceptions_v2_id id TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_exceptions_v2_stack_trace stack_trace TYPE tokenbf_v1(10240, 3, 0) GRANULARITY 4,
    INDEX idx_exceptions_v2_trace_id trace_id TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_exceptions_v2_linked_trace_id linked_trace_id TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_exceptions_v2_session_id session_id TYPE bloom_filter(0.001) GRANULARITY 1
) ENGINE = MergeTree
PARTITION BY toYYYYMMDD(recorded_at)
ORDER BY (project_id, recorded_at, exception_hash)
