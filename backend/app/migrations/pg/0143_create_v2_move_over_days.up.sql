CREATE TABLE IF NOT EXISTS v2_move_over_days (
    source_table TEXT NOT NULL,
    day TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('started', 'done')),
    moved_rows BIGINT NOT NULL DEFAULT 0 CHECK (moved_rows >= 0),
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (source_table, day)
);
