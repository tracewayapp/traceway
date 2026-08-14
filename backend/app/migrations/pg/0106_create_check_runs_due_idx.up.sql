CREATE INDEX IF NOT EXISTS check_runs_due_idx ON check_runs (status, scheduled_for)
