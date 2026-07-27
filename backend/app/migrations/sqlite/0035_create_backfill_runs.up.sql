CREATE TABLE IF NOT EXISTS backfill_runs (
    name TEXT PRIMARY KEY,
    ran_at DATETIME NOT NULL
);
