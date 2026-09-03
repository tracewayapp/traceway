DROP INDEX IF EXISTS idx_ai_traces_project_conversation;
CREATE INDEX IF NOT EXISTS idx_ai_traces_project_conversation_recorded ON ai_traces(project_id, conversation_id, recorded_at);
