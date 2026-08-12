ALTER TABLE ai_traces ADD COLUMN conversation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_traces ADD COLUMN tool_call_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ai_traces ADD COLUMN tool_names TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_traces ADD COLUMN flagged INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ai_traces ADD COLUMN flagged_terms TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_ai_traces_project_conversation ON ai_traces(project_id, conversation_id);
