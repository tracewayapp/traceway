ALTER TABLE sessions RENAME COLUMN distributed_trace_id TO trace_id;

ALTER TABLE sessions ALTER COLUMN trace_id SET DEFAULT '';

UPDATE sessions SET trace_id = CASE
    WHEN regexp_full_match(lower(replace(COALESCE(trace_id, ''), '-', '')), '[0-9a-f]{32}') AND lower(replace(COALESCE(trace_id, ''), '-', '')) != '00000000000000000000000000000000'
    THEN lower(replace(COALESCE(trace_id, ''), '-', '')) ELSE '' END;

ALTER TABLE endpoints_v2 DROP COLUMN linked_trace_id;

ALTER TABLE tasks_v2 DROP COLUMN linked_trace_id;

ALTER TABLE ai_traces_v2 DROP COLUMN linked_trace_id;

ALTER TABLE exceptions_v2 DROP COLUMN linked_trace_id;

ALTER TABLE profiles DROP COLUMN distributed_trace_id;
