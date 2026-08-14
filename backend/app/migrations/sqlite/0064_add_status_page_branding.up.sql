ALTER TABLE status_pages ADD COLUMN description TEXT NOT NULL DEFAULT '';

ALTER TABLE status_pages ADD COLUMN logo_key TEXT NOT NULL DEFAULT '';

ALTER TABLE status_pages ADD COLUMN custom_domain TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS status_pages_custom_domain_unique ON status_pages (custom_domain) WHERE custom_domain <> '';
