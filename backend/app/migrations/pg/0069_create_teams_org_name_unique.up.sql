CREATE UNIQUE INDEX IF NOT EXISTS teams_org_name_unique ON teams (organization_id, LOWER(name))
