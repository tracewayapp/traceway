CREATE INDEX IF NOT EXISTS idx_agent_write_events_org_created ON agent_write_events(organization_id, created_at)
