ALTER TABLE spans ADD INDEX idx_entity_id entity_id TYPE bloom_filter GRANULARITY 4
