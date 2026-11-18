CREATE TABLE IF NOT EXISTS slo_buckets (
    project_id UUID,
    slo_id Int64,
    bucket_start DateTime,
    good Int64,
    total Int64,
    computed_at DateTime64(3)
) ENGINE = ReplacingMergeTree(computed_at)
PARTITION BY toYYYYMM(bucket_start)
ORDER BY (project_id, slo_id, bucket_start)
