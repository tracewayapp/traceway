-- No ART index on purpose: it would serialize the appender and DuckDB's
-- per-row-group zonemaps already prune time windows because batches arrive
-- in time order.
CREATE TABLE IF NOT EXISTS points (
    series_id UBIGINT NOT NULL,
    ts TIMESTAMP NOT NULL,
    value DOUBLE NOT NULL
);

CREATE TABLE IF NOT EXISTS series (
    series_id UBIGINT NOT NULL,
    name VARCHAR NOT NULL,
    tags MAP(VARCHAR, VARCHAR)
);
