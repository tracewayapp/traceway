INSERT INTO exception_stack_traces (id, project_id, trace_id, trace_type, exception_hash, stack_trace, recorded_at, scope, app_version, server_name, is_message)
SELECT id, project_id, transaction_id, transaction_type, exception_hash, stack_trace, recorded_at, scope, app_version, server_name, is_message
FROM exception_stack_traces_old;
