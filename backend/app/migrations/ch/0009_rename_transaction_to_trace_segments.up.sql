ALTER TABLE segments RENAME COLUMN transaction_id TO trace_id;
ALTER TABLE segments DROP INDEX idx_transaction_id;
ALTER TABLE segments ADD INDEX idx_trace_id trace_id TYPE bloom_filter(0.001) GRANULARITY 1;
