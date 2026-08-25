-- Traceway's current metric_points layout, kept as the baseline.
CREATE TABLE IF NOT EXISTS metric_points (
    name LowCardinality(String),
    value Float64,
    tags Map(LowCardinality(String), String),
    recorded_at DateTime64(3),
    INDEX idx_tags_keys mapKeys(tags) TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_tags_values mapValues(tags) TYPE bloom_filter(0.01) GRANULARITY 4
) ENGINE = MergeTree
PARTITION BY toYYYYMMDD(recorded_at)
ORDER BY (name, recorded_at)
SETTINGS index_granularity = 8192;
