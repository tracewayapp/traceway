ALTER TABLE metric_histograms_1h_mv MODIFY QUERY
SELECT
    project_id,
    name,
    toStartOfHour(source.recorded_at) AS recorded_at,
    release, env, os, os_major, client, client_major, form_factor, device_class, network, country, view,
    mapSort(source.tags) AS tags,
    sum(cnt) AS cnt,
    sum(if(source.weight_squares > 0, source.weight_squares, source.cnt * 1000)) AS weight_squares,
    sum(total) AS total,
    min(min_val) AS min_val,
    max(max_val) AS max_val,
    sum(zero_cnt) AS zero_cnt,
    sumMap(buckets.1, buckets.2) AS buckets
FROM metric_histograms AS source
GROUP BY project_id, name, toStartOfHour(source.recorded_at), release, env, os, os_major, client, client_major, form_factor, device_class, network, country, view, mapSort(source.tags)
