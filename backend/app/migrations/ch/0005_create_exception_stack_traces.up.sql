CREATE TABLE IF NOT EXISTS exception_stack_traces
(
    `id` UUID,
    `project_id` UUID,
    `transaction_id` Nullable(UUID),
    `transaction_type` LowCardinality(String) DEFAULT 'endpoint',
    `exception_hash` String,
    `stack_trace` String,
    `recorded_at` DateTime,
    `scope` String DEFAULT '{}',
    `app_version` LowCardinality(String) DEFAULT '',
    `server_name` LowCardinality(String) DEFAULT '',
    `is_message` UInt8 DEFAULT 0,
    INDEX idx_exception_hash exception_hash TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_transaction_id transaction_id TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_is_message is_message TYPE set(2) GRANULARITY 1,
    INDEX idx_id id TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_stack_trace stack_trace TYPE tokenbf_v1(10240, 3, 0) GRANULARITY 4
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(recorded_at)
ORDER BY (project_id, recorded_at, exception_hash)
SETTINGS index_granularity = 8192
