ALTER TABLE ai_traces ADD INDEX idx_conversation_id conversation_id TYPE bloom_filter(0.001) GRANULARITY 1
