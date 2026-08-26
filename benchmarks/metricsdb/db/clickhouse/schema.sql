-- Normalized layout: a narrow fact table ordered by (series_id, ts) so each
-- series is a contiguous run, plus a small dimension table for identity.
-- Timestamps and ids compress to almost nothing under DoubleDelta/Delta; the
-- value column is where all the bytes are, and on this data Gorilla loses to
-- Delta + ZSTD(3) by a third (measured: 81 MB vs 52 MB for 20M points, with
-- ZSTD(6)/(9) buying 3-4% more for 2-3x the CPU).
-- The minmax index on ts is what lets a time-window query without a series
-- filter (discovery: every series active in the last hour) skip granules; the
-- primary key alone cannot, so it scanned the whole table and grew linearly.
CREATE TABLE IF NOT EXISTS points (
    series_id UInt64 CODEC(Delta(8), ZSTD(1)),
    ts DateTime64(3) CODEC(DoubleDelta, ZSTD(1)),
    value Float64 CODEC(Delta(8), ZSTD(3)),
    INDEX idx_ts ts TYPE minmax GRANULARITY 4
) ENGINE = MergeTree
PARTITION BY toDate(ts)
ORDER BY (series_id, ts)
SETTINGS index_granularity = 8192, old_parts_lifetime = 30;

CREATE TABLE IF NOT EXISTS series (
    series_id UInt64,
    name LowCardinality(String),
    tags Map(LowCardinality(String), String)
) ENGINE = ReplacingMergeTree
ORDER BY series_id;
