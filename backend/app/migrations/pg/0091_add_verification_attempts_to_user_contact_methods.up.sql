ALTER TABLE user_contact_methods ADD COLUMN IF NOT EXISTS verification_attempts INT NOT NULL DEFAULT 0
