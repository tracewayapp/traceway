CREATE INDEX IF NOT EXISTS synthetic_checks_due_idx ON synthetic_checks (enabled, next_run_at)
