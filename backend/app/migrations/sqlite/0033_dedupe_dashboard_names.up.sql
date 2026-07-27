UPDATE dashboards SET name = substr(name, 1, 100 - length(CAST(id AS TEXT)) - 1) || ' ' || CAST(id AS TEXT)
WHERE EXISTS (SELECT 1 FROM dashboards d2 WHERE d2.organization_id = dashboards.organization_id AND LOWER(d2.name) = LOWER(dashboards.name) AND d2.id < dashboards.id)
