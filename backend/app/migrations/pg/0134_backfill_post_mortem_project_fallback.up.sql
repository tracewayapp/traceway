UPDATE post_mortems SET project_id = (SELECT p.id FROM projects p WHERE p.organization_id = post_mortems.organization_id ORDER BY p.created_at LIMIT 1) WHERE project_id IS NULL
