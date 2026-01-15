ALTER TABLE exception_stack_traces ADD COLUMN IF NOT EXISTS transaction_type String DEFAULT 'endpoint';
