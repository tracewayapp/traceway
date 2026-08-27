-- Codec-tuned variant of db/firebolt/schema.sql. Same layout (narrow fact
-- table + dimension table, PRIMARY INDEX (series_id, ts), daily partitions);
-- the difference is per-column codec chains instead of table-level ZSTD,
-- mirroring the tuned ClickHouse schema: transform codec first, entropy
-- codec second. Delta everywhere: ts is a fixed-interval scrape clock
-- (constant deltas, ZSTD flattens them), series_id arrives in long sorted
-- runs, and on this corpus the ClickHouse measurement preferred Delta over
-- Gorilla for value by a third. The live engine gates DoubleDelta and
-- Gorilla behind per-codec feature flags that are off in the current dev
-- build ("Unsupported compression type"), and rejects codec width
-- parameters (width defaults to sizeof(type)) - Delta + ZSTD is the
-- strongest chain the deployed build accepts.
CREATE FACT TABLE IF NOT EXISTS points (
    series_id BIGINT NOT NULL COMPRESSION (Delta, ZSTD(3)),
    ts TIMESTAMPNTZ NOT NULL COMPRESSION (Delta, ZSTD(3)),
    value DOUBLE PRECISION NOT NULL COMPRESSION (Delta, ZSTD(3))
) PRIMARY INDEX series_id, ts
PARTITION BY DATE_TRUNC('day', ts);

CREATE TABLE IF NOT EXISTS series (
    series_id BIGINT NOT NULL COMPRESSION (Delta, ZSTD(3)),
    name TEXT NOT NULL COMPRESSION (ZSTD(3)),
    tags TEXT NOT NULL COMPRESSION (ZSTD(3))
) PRIMARY INDEX series_id;
