DROP INDEX IF EXISTS idx_endpoints_project_endpoint;
CREATE INDEX IF NOT EXISTS idx_endpoints_project_endpoint_recorded ON endpoints(project_id, endpoint, recorded_at);
DROP INDEX IF EXISTS idx_ai_traces_project_trace_name;
CREATE INDEX IF NOT EXISTS idx_ai_traces_project_trace_name_recorded ON ai_traces(project_id, trace_name, recorded_at);
CREATE INDEX IF NOT EXISTS idx_tasks_project_task_name_recorded ON tasks(project_id, task_name, recorded_at);
