ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'traceway:read'
