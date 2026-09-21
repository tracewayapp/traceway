ALTER TABLE exceptions_v2
    DROP INDEX IF EXISTS idx_exceptions_v2_linked_trace_id,
    DROP COLUMN IF EXISTS linked_trace_id
