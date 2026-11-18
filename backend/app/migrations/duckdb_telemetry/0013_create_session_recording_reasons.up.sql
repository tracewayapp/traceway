CREATE TABLE session_recording_reasons (
 project_id VARCHAR NOT NULL,
 session_id VARCHAR NOT NULL,
 trigger_id VARCHAR NOT NULL,
 revision VARCHAR NOT NULL,
 name VARCHAR NOT NULL,
 sequence VARCHAR NOT NULL,
 fired_at BIGINT NOT NULL,
 PRIMARY KEY (project_id, trigger_id, session_id, revision)
);
