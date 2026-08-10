ALTER TABLE user_contact_methods ADD COLUMN IF NOT EXISTS verification_code_hash VARCHAR(64) NOT NULL DEFAULT ''
