CREATE TABLE session_recording_reasons (
 project_id UUID,
 session_id UUID,
 trigger_id String,
 revision String,
 name String,
 sequence String,
 fired_at Int64,
 recorded_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree
PARTITION BY toYYYYMM(recorded_at)
ORDER BY (project_id, trigger_id, session_id, revision)
TTL recorded_at + INTERVAL 30 DAY;
