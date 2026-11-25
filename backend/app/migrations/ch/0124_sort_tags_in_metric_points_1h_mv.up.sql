ALTER TABLE metric_points_1h_mv MODIFY QUERY
SELECT
    project_id,
    name,
    mapSort(source.tags) AS tags,
    toStartOfHour(source.recorded_at) AS recorded_at,
    minState(value) AS min_val,
    maxState(value) AS max_val,
    sumState(value) AS sum_val,
    countState() AS count_val
FROM metric_points AS source
GROUP BY project_id, name, mapSort(source.tags), toStartOfHour(source.recorded_at)
