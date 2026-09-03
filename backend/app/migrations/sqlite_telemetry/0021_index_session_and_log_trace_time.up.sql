CREATE INDEX IF NOT EXISTS idx_exceptions_session_recorded ON exception_stack_traces(project_id, session_id, recorded_at) WHERE session_id IS NOT NULL;
DROP INDEX IF EXISTS idx_exceptions_session;
CREATE INDEX IF NOT EXISTS idx_log_records_project_trace_timestamp ON log_records(project_id, trace_id, timestamp);
DROP INDEX IF EXISTS idx_log_records_project_trace;
