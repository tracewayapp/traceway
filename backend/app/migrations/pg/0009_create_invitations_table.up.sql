CREATE TABLE invitations (
    id SERIAL PRIMARY KEY,
    organization_id INT NOT NULL REFERENCES organizations(id),
    email TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin','user','readonly')),
    token TEXT NOT NULL UNIQUE,
    invited_by INT NOT NULL REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','expired')),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
)
