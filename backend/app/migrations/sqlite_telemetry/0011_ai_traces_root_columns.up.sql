ALTER TABLE ai_traces ADD COLUMN trace_id TEXT DEFAULT NULL;
ALTER TABLE ai_traces ADD COLUMN span_id TEXT DEFAULT NULL;
ALTER TABLE ai_traces ADD COLUMN parent_span_id TEXT DEFAULT NULL;
ALTER TABLE ai_traces ADD COLUMN distributed_trace_id TEXT DEFAULT NULL;
UPDATE ai_traces SET trace_id = id WHERE trace_id IS NULL;
