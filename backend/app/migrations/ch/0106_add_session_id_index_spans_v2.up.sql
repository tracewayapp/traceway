ALTER TABLE spans_v2 ADD INDEX IF NOT EXISTS idx_session_id session_id TYPE bloom_filter(0.001) GRANULARITY 1
