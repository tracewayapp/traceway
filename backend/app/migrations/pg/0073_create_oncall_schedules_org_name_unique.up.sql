CREATE UNIQUE INDEX IF NOT EXISTS oncall_schedules_org_name_unique ON oncall_schedules (organization_id, LOWER(name))
