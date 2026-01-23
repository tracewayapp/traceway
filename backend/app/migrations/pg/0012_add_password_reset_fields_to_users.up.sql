ALTER TABLE users
ADD COLUMN password_reset_token TEXT,
ADD COLUMN password_reset_expires_at TIMESTAMPTZ,
ADD COLUMN password_reset_requested_at TIMESTAMPTZ
