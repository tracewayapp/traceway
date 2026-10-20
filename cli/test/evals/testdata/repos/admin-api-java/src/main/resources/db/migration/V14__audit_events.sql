CREATE TABLE audit_events (
    id          BIGSERIAL PRIMARY KEY,
    org_id      BIGINT       NOT NULL REFERENCES organizations (id),
    actor_id    BIGINT       NOT NULL REFERENCES users (id),
    action      VARCHAR(64)  NOT NULL,
    target_type VARCHAR(64)  NOT NULL,
    target_id   VARCHAR(64),
    ip_address  INET,
    user_agent  TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Every read filters by organization and lists newest first.
CREATE INDEX audit_events_org_created_idx ON audit_events (org_id, created_at DESC);
