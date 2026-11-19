ALTER TABLE authorization_codes ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'traceway:read'
