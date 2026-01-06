CREATE TABLE IF NOT EXISTS projects (
    id String,
    name String,
    token String,
    created_at DateTime DEFAULT now()
) ENGINE = MergeTree()
ORDER BY (created_at, id)
