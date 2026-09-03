DROP INDEX IF EXISTS idx_exceptions_project_hash;
CREATE INDEX IF NOT EXISTS idx_exceptions_project_hash_recorded ON exception_stack_traces(project_id, exception_hash, recorded_at);
