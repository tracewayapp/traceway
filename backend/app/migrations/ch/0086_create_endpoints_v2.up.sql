CREATE TABLE IF NOT EXISTS endpoints_v2 (
    id UUID,
    project_id UUID,
    trace_id String DEFAULT '',
    span_id String DEFAULT '',
    parent_span_id String DEFAULT '',
    linked_trace_id String DEFAULT '',
    endpoint LowCardinality(String),
    duration Int64 DEFAULT 0,
    recorded_at DateTime64(6),
    status_code Int16 DEFAULT 0,
    body_size Int32 DEFAULT 0,
    client_ip String DEFAULT '',
    attributes String DEFAULT '{}',
    app_version LowCardinality(String) DEFAULT '',
    server_name LowCardinality(String) DEFAULT '',
    is_stream UInt8 DEFAULT 0,
    is_root UInt8 DEFAULT 1,
    INDEX idx_endpoints_v2_endpoint endpoint TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_endpoints_v2_status_code status_code TYPE set(100) GRANULARITY 4,
    INDEX idx_endpoints_v2_id id TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_endpoints_v2_trace_id trace_id TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_endpoints_v2_linked_trace_id linked_trace_id TYPE bloom_filter(0.001) GRANULARITY 1
) ENGINE = MergeTree
PARTITION BY toYYYYMMDD(recorded_at)
ORDER BY (project_id, recorded_at, endpoint)
