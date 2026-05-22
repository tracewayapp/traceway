ALTER TABLE spans ADD COLUMN entity_id TEXT DEFAULT NULL;
UPDATE spans SET entity_id = trace_id WHERE entity_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_spans_project_entity ON spans(project_id, entity_id);
