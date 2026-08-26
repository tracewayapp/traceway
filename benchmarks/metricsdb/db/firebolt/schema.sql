-- Firebolt sorts each tablet on the primary index and prunes with a sparse
-- index plus per-tablet min/max, so (series_id, ts) gives range scans for
-- one metric and daily partitions bound the 24h query.
-- ZSTD instead of the LZ4 default: 39% smaller on this data (measured 75 MB
-- vs 124 MB for 20M points); level 6 gains another 1% for more CPU.
CREATE FACT TABLE IF NOT EXISTS points (
    series_id BIGINT NOT NULL,
    ts TIMESTAMPNTZ NOT NULL,
    value DOUBLE PRECISION NOT NULL
) PRIMARY INDEX series_id, ts
PARTITION BY DATE_TRUNC('day', ts)
WITH (compression = ZSTD, compression_level = 3);

-- Firebolt Core has no DIMENSION tables; a small fact table with its own
-- primary index plays that role.
CREATE TABLE IF NOT EXISTS series (
    series_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    tags TEXT NOT NULL
) PRIMARY INDEX series_id
WITH (compression = ZSTD, compression_level = 3);
