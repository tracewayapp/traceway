INSERT INTO segments (id, trace_id, project_id, name, start_time, duration, recorded_at)
SELECT id, transaction_id, project_id, name, start_time, duration, recorded_at
FROM segments_old;
