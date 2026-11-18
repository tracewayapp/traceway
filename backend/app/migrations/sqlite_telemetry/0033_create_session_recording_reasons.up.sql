CREATE TABLE session_recording_reasons (
 project_id TEXT NOT NULL,
 session_id TEXT NOT NULL,
 trigger_id TEXT NOT NULL,
 revision TEXT NOT NULL,
 name TEXT NOT NULL,
 sequence TEXT NOT NULL,
 fired_at INTEGER NOT NULL,
 PRIMARY KEY (project_id, trigger_id, session_id, revision)
);
