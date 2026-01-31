ALTER TABLE exception_stack_traces RENAME COLUMN transaction_id TO trace_id;
ALTER TABLE exception_stack_traces RENAME COLUMN transaction_type TO trace_type;
ALTER TABLE exception_stack_traces DROP INDEX idx_transaction_id;
ALTER TABLE exception_stack_traces ADD INDEX idx_trace_id trace_id TYPE bloom_filter(0.01) GRANULARITY 4;
