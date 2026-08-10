CREATE UNIQUE INDEX IF NOT EXISTS escalation_policies_org_name_unique ON escalation_policies (organization_id, LOWER(name))
