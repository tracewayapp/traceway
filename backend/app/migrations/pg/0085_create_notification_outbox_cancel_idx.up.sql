CREATE INDEX IF NOT EXISTS notification_outbox_cancel_idx ON notification_outbox (cancel_key) WHERE cancel_key <> ''
