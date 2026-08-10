CREATE UNIQUE INDEX IF NOT EXISTS pages_dedup_open_unique ON pages (dedup_key) WHERE status <> 'resolved'
