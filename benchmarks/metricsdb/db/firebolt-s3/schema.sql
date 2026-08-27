-- Codec-tuned variant of db/firebolt/schema.sql. Same layout (narrow fact
-- table + dimension table, PRIMARY INDEX (series_id, ts), daily partitions);
-- the difference is per-column codec chains instead of table-level ZSTD,
-- mirroring the tuned ClickHouse schema: transform codec first (Delta /
-- DoubleDelta / Gorilla), entropy codec second. ts is a fixed-interval
-- scrape clock so DoubleDelta collapses it; series_id arrives in long
-- sorted runs so Delta feeds ZSTD tiny residuals; value gets Gorilla
-- (XOR-based float codec, the VictoriaMetrics approach). Note the ClickHouse
-- measurement on this corpus preferred Delta over Gorilla for value - if the
-- smoke run shows weak value compression, A/B that column the same way.
CREATE FACT TABLE IF NOT EXISTS points (
    series_id BIGINT NOT NULL COMPRESSION (Delta, ZSTD(3)),
    ts TIMESTAMPNTZ NOT NULL COMPRESSION (DoubleDelta, ZSTD(3)),
    value DOUBLE PRECISION NOT NULL COMPRESSION (Gorilla, ZSTD(3))
) PRIMARY INDEX series_id, ts
PARTITION BY DATE_TRUNC('day', ts);

CREATE TABLE IF NOT EXISTS series (
    series_id BIGINT NOT NULL COMPRESSION (Delta, ZSTD(3)),
    name TEXT NOT NULL COMPRESSION (ZSTD(3)),
    tags TEXT NOT NULL COMPRESSION (ZSTD(3))
) PRIMARY INDEX series_id;
