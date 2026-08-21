CREATE INDEX IF NOT EXISTS pages_org_active_idx ON pages (organization_id, created_at DESC) WHERE status <> 'resolved'
