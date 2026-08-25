-- Normalized layout: a narrow fact table ordered by (series_id, ts) so each
-- series is a contiguous run and the codecs see monotonic timestamps and
-- slowly-changing values, plus a small dimension table for identity.
CREATE TABLE IF NOT EXISTS points (
    series_id UInt64 CODEC(Delta(8), ZSTD(1)),
    ts DateTime64(3) CODEC(DoubleDelta, ZSTD(1)),
    value Float64 CODEC(Gorilla, ZSTD(1))
) ENGINE = MergeTree
PARTITION BY toDate(ts)
ORDER BY (series_id, ts)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS series (
    series_id UInt64,
    name LowCardinality(String),
    tags Map(LowCardinality(String), String)
) ENGINE = ReplacingMergeTree
ORDER BY series_id;
