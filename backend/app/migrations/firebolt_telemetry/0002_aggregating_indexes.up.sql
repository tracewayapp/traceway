CREATE AGGREGATING INDEX IF NOT EXISTS endpoints_dash_idx ON endpoints (
    project_id, DATE_TRUNC('minute', recorded_at), endpoint, is_stream, is_root,
    COUNT(*), AVG(duration), MAX(recorded_at),
    SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END),
    SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END),
    SUM(CASE WHEN duration <= 750000000 AND status_code < 500 THEN 1 ELSE 0 END),
    SUM(CASE WHEN duration > 750000000 AND duration <= 1500000000 AND status_code < 500 THEN 1 ELSE 0 END),
    SUM(CASE WHEN duration > 1500000000 OR status_code >= 500 THEN 1 ELSE 0 END),
    SUM(CASE WHEN duration <= 500000000 AND status_code < 500 THEN 1 ELSE 0 END),
    SUM(CASE WHEN duration > 500000000 AND duration <= 2000000000 AND status_code < 500 THEN 1 ELSE 0 END),
    MAX(is_stream), MAX(is_root), MAX(CASE WHEN is_root = 0 THEN 1 ELSE 0 END),
    PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY duration),
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration),
    PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration)
);

CREATE AGGREGATING INDEX IF NOT EXISTS tasks_dash_idx ON tasks (
    project_id, DATE_TRUNC('minute', recorded_at), task_name, is_root,
    COUNT(*), AVG(duration), MAX(recorded_at),
    MAX(is_root), MAX(CASE WHEN is_root = 0 THEN 1 ELSE 0 END),
    PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY duration),
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration),
    PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration)
);

CREATE AGGREGATING INDEX IF NOT EXISTS metric_points_dash_idx ON metric_points (
    project_id, name, DATE_TRUNC('minute', recorded_at), JSON_POINTER_EXTRACT_TEXT(tags, '/host'), server_name,
    COUNT(*), AVG(value), MIN(value), MAX(value), SUM(value), MAX(recorded_at)
);

CREATE AGGREGATING INDEX IF NOT EXISTS log_records_count_idx ON log_records (
    project_id, DATE_TRUNC('minute', timestamp), severity_text, service_name,
    COUNT(*)
);

CREATE AGGREGATING INDEX IF NOT EXISTS exceptions_group_idx ON exception_stack_traces (
    project_id, DATE_TRUNC('minute', recorded_at), exception_hash, is_message, trace_type,
    COUNT(*), MAX(recorded_at), MIN(recorded_at)
);
