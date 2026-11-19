CREATE TABLE IF NOT EXISTS metric_histograms_1h (
    `project_id` UUID,
    `name` LowCardinality(String),
    `recorded_at` DateTime,
    `release` LowCardinality(String),
    `env` LowCardinality(String),
    `os` LowCardinality(String),
    `os_major` LowCardinality(String),
    `client` LowCardinality(String),
    `client_major` LowCardinality(String),
    `form_factor` LowCardinality(String),
    `device_class` LowCardinality(String),
    `network` LowCardinality(String),
    `country` LowCardinality(String),
    `view` LowCardinality(String),
    `tags` Map(LowCardinality(String), String),
    `cnt` SimpleAggregateFunction(sum, Float64),
    `total` SimpleAggregateFunction(sum, Float64),
    `min_val` SimpleAggregateFunction(min, Float64),
    `max_val` SimpleAggregateFunction(max, Float64),
    `zero_cnt` SimpleAggregateFunction(sum, Float64),
    `buckets` SimpleAggregateFunction(sumMap, Tuple(Array(Int32), Array(Float64)))
) ENGINE = AggregatingMergeTree
PARTITION BY toYYYYMM(recorded_at)
ORDER BY (project_id, name, recorded_at, release, env, os, os_major, client, client_major, form_factor, device_class, network, country, view, tags)
SETTINGS index_granularity = 8192
