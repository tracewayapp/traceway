CREATE UNIQUE INDEX idx_invitations_email_org_pending ON invitations(email, organization_id) WHERE status = 'pending'
