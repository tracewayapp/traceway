CREATE UNIQUE INDEX IF NOT EXISTS page_notifications_ack_token_unique ON page_notifications (ack_token_hash) WHERE ack_token_hash <> ''
