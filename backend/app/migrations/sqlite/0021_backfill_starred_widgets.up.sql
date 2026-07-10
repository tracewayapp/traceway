INSERT INTO starred_widgets (widget_id, position, col_span, size, created_at)
SELECT wgw.id, ROW_NUMBER() OVER (PARTITION BY wg.project_id ORDER BY wgw.updated_at DESC) - 1, 1, 'sm', datetime('now')
FROM widget_group_widgets wgw
JOIN widget_groups wg ON wg.id = wgw.widget_group_id
WHERE wgw.is_starred = 1;
