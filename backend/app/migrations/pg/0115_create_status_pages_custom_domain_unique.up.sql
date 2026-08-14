CREATE UNIQUE INDEX IF NOT EXISTS status_pages_custom_domain_unique ON status_pages (custom_domain) WHERE custom_domain <> ''
