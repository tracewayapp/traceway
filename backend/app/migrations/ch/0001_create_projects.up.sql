CREATE TABLE IF NOT EXISTS projects
(
    `id` UUID,
    `name` String,
    `token` String,
    `framework` LowCardinality(String) DEFAULT 'custom',
    `created_at` DateTime DEFAULT now(),
    INDEX idx_token token TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_id id TYPE bloom_filter(0.001) GRANULARITY 1
)
ENGINE = MergeTree
ORDER BY id
SETTINGS index_granularity = 8192
