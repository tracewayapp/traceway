ALTER TABLE endpoints ADD COLUMN trace_id TEXT DEFAULT NULL;
ALTER TABLE endpoints ADD COLUMN parent_span_id TEXT DEFAULT NULL;
UPDATE endpoints SET trace_id = id WHERE trace_id IS NULL;
