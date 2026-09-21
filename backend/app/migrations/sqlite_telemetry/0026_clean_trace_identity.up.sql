ALTER TABLE sessions ADD COLUMN trace_id TEXT NOT NULL DEFAULT '';

UPDATE sessions SET trace_id = CASE
    WHEN length(lower(replace(COALESCE(distributed_trace_id, ''), '-', ''))) = 32 AND lower(replace(COALESCE(distributed_trace_id, ''), '-', '')) NOT GLOB '*[^0-9a-f]*' AND lower(replace(COALESCE(distributed_trace_id, ''), '-', '')) != '00000000000000000000000000000000'
    THEN lower(replace(COALESCE(distributed_trace_id, ''), '-', '')) ELSE '' END;

ALTER TABLE sessions DROP COLUMN distributed_trace_id;

DROP INDEX IF EXISTS idx_endpoints_v2_linked_trace_id;

ALTER TABLE endpoints_v2 DROP COLUMN linked_trace_id;

DROP INDEX IF EXISTS idx_tasks_v2_linked_trace_id;

ALTER TABLE tasks_v2 DROP COLUMN linked_trace_id;

DROP INDEX IF EXISTS idx_ai_traces_v2_linked_trace_id;

ALTER TABLE ai_traces_v2 DROP COLUMN linked_trace_id;

DROP INDEX IF EXISTS idx_exceptions_v2_linked_trace_id;

ALTER TABLE exceptions_v2 DROP COLUMN linked_trace_id;

ALTER TABLE profiles DROP COLUMN distributed_trace_id;
