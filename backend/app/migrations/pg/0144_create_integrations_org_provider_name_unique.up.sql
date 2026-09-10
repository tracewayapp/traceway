CREATE UNIQUE INDEX IF NOT EXISTS integrations_org_provider_name_unique ON integrations (organization_id, provider, name)
