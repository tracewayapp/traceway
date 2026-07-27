CREATE UNIQUE INDEX IF NOT EXISTS dashboards_org_name_unique ON dashboards (organization_id, LOWER(name))
