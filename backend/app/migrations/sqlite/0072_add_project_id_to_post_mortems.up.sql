ALTER TABLE post_mortems ADD COLUMN project_id TEXT REFERENCES projects(id) ON DELETE CASCADE;

UPDATE post_mortems SET project_id = (SELECT sc.project_id FROM check_incidents ci JOIN synthetic_checks sc ON sc.id = ci.check_id WHERE ci.id = post_mortems.incident_id) WHERE incident_id IS NOT NULL;

UPDATE post_mortems SET project_id = (SELECT p.id FROM projects p WHERE p.organization_id = post_mortems.organization_id ORDER BY p.created_at LIMIT 1) WHERE project_id IS NULL;

CREATE INDEX IF NOT EXISTS post_mortems_project_idx ON post_mortems (project_id, updated_at);
