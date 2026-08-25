-- Firebolt sorts each tablet on the primary index and prunes with a sparse
-- index plus per-tablet min/max, so (series_id, ts) gives range scans for
-- one metric and daily partitions bound the 24h query.
CREATE FACT TABLE IF NOT EXISTS points (
    series_id BIGINT NOT NULL,
    ts TIMESTAMPNTZ NOT NULL,
    value DOUBLE PRECISION NOT NULL
) PRIMARY INDEX series_id, ts
PARTITION BY DATE_TRUNC('day', ts);

-- Firebolt Core has no DIMENSION tables; a small fact table with its own
-- primary index plays that role.
CREATE TABLE IF NOT EXISTS series (
    series_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    tags TEXT NOT NULL
) PRIMARY INDEX series_id;
