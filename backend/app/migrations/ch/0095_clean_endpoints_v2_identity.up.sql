ALTER TABLE endpoints_v2
    DROP INDEX IF EXISTS idx_endpoints_v2_linked_trace_id,
    DROP COLUMN IF EXISTS linked_trace_id
