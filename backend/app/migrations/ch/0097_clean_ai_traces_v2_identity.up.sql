ALTER TABLE ai_traces_v2
    DROP INDEX IF EXISTS idx_ai_traces_v2_linked_trace_id,
    DROP COLUMN IF EXISTS linked_trace_id
