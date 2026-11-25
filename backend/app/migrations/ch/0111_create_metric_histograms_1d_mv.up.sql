CREATE MATERIALIZED VIEW IF NOT EXISTS metric_histograms_1d_mv TO metric_histograms_1d AS
SELECT
    project_id,
    name,
    toStartOfDay(recorded_at) AS recorded_at,
    release, env, os, os_major, client, client_major, form_factor, device_class, network, country, view,
    tags,
    sum(cnt) AS cnt,
    sum(total) AS total,
    min(min_val) AS min_val,
    max(max_val) AS max_val,
    sum(zero_cnt) AS zero_cnt,
    sumMap(buckets.1, buckets.2) AS buckets
FROM metric_histograms
GROUP BY project_id, name, recorded_at, release, env, os, os_major, client, client_major, form_factor, device_class, network, country, view, tags
