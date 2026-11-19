ALTER TABLE personal_access_tokens ADD COLUMN scope TEXT NOT NULL DEFAULT 'traceway:read';
ALTER TABLE refresh_tokens ADD COLUMN scope TEXT NOT NULL DEFAULT 'traceway:read';
ALTER TABLE authorization_codes ADD COLUMN scope TEXT NOT NULL DEFAULT 'traceway:read';
