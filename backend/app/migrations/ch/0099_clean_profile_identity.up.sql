ALTER TABLE profiles
    DROP INDEX IF EXISTS idx_distributed_trace_id,
    DROP COLUMN IF EXISTS distributed_trace_id
