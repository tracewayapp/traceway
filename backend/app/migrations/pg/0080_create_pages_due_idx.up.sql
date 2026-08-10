CREATE INDEX IF NOT EXISTS pages_due_idx ON pages (status, next_escalation_at)
