CREATE TABLE IF NOT EXISTS slo_buckets (
    project_id VARCHAR NOT NULL,
    slo_id BIGINT NOT NULL,
    bucket_start TIMESTAMP NOT NULL,
    good BIGINT NOT NULL DEFAULT 0,
    total BIGINT NOT NULL DEFAULT 0,
    computed_at TIMESTAMP NOT NULL,
    UNIQUE(project_id, slo_id, bucket_start)
);
