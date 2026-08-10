CREATE INDEX IF NOT EXISTS notification_outbox_due_idx ON notification_outbox (status, next_attempt_at)
