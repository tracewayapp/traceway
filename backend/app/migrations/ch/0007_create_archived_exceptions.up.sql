CREATE TABLE IF NOT EXISTS archived_exceptions
(
    `project_id` UUID,
    `exception_hash` String,
    `archived_at` DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(archived_at)
ORDER BY (project_id, exception_hash)
SETTINGS index_granularity = 8192
