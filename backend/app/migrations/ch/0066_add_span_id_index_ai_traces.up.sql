ALTER TABLE ai_traces ADD INDEX idx_span_id span_id TYPE bloom_filter GRANULARITY 4
