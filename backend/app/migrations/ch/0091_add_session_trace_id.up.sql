ALTER TABLE sessions ADD COLUMN IF NOT EXISTS trace_id String DEFAULT
    ifNull(nullIf(replaceAll(toString(distributed_trace_id), '-', ''), '00000000000000000000000000000000'), '')
