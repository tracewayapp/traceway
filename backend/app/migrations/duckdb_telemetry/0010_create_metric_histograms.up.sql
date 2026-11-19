CREATE TABLE IF NOT EXISTS metric_histograms (
    project_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    recorded_at TIMESTAMP NOT NULL,
    grain_seconds BIGINT NOT NULL DEFAULT 60,
    dims VARCHAR NOT NULL DEFAULT '{}',
    cnt DOUBLE NOT NULL DEFAULT 0,
    total DOUBLE NOT NULL DEFAULT 0,
    min_val DOUBLE NOT NULL DEFAULT 0,
    max_val DOUBLE NOT NULL DEFAULT 0,
    zero_cnt DOUBLE NOT NULL DEFAULT 0,
    buckets VARCHAR NOT NULL DEFAULT '{}'
);
CREATE TABLE IF NOT EXISTS metric_exemplars (
    project_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    recorded_at TIMESTAMP NOT NULL,
    dims VARCHAR NOT NULL DEFAULT '{}',
    value DOUBLE NOT NULL,
    session_id VARCHAR NOT NULL
);
