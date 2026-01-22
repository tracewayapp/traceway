ALTER TABLE projects ADD COLUMN organization_id INT REFERENCES organizations(id)
