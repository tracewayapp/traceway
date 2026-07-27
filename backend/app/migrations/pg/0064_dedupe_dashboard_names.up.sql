UPDATE dashboards SET name = LEFT(name, 100 - LENGTH(id::text) - 1) || ' ' || id::text
WHERE EXISTS (SELECT 1 FROM dashboards d2 WHERE d2.organization_id = dashboards.organization_id AND LOWER(d2.name) = LOWER(dashboards.name) AND d2.id < dashboards.id)
