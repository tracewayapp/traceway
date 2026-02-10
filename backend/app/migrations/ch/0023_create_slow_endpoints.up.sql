CREATE TABLE IF NOT EXISTS slow_endpoints
(
    `project_id` UUID,
    `endpoint` LowCardinality(String),
    `offset_ms` UInt32 DEFAULT 0,
    `reason` String DEFAULT ''
)
ENGINE = ReplacingMergeTree()
ORDER BY (project_id, endpoint)
