-- Earlier releases kept only the escalation page for a firing series.
WITH open_pages AS (
    SELECT r.id AS rule_id, p.created_at, p.dedup_key, p.body, r.config->>'metricName' AS metric,
           substr(p.dedup_key, length(CAST(r.id AS TEXT)) + 2) AS page_token
    FROM pages p
    JOIN notification_rules r ON r.id = p.rule_id
    JOIN notification_channels c ON c.id = r.channel_id
    WHERE p.status <> 'resolved' AND p.rule_type = 'metric_threshold'
      AND r.rule_type = 'metric_threshold' AND c.channel_type = 'escalation'
      AND jsonb_typeof(r.config->'groupBy') = 'array' AND r.config->'groupBy' <> '[]'::jsonb
), series_pages AS (
    SELECT rule_id, created_at, metric, dedup_key,
           CASE
               WHEN length(page_token) = 32 AND page_token NOT LIKE '%:%' AND page_token NOT LIKE '%{%'
                   THEN CASE
                       -- Long page keys are digests. The original label ends at the last closing brace in the alert body.
                       WHEN body LIKE 'The metric %' THEN rtrim(substr(body, 12), 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 .+-')
                       WHEN body LIKE '% more series of the metric %' THEN metric || ':overflow'
                   END
               ELSE page_token
           END AS token
    FROM open_pages
)
INSERT INTO notification_rule_firing (rule_id, token, page_key, since)
SELECT rule_id, token, dedup_key, created_at
FROM series_pages
WHERE token = metric || ':overflow'
   OR (substr(token, 1, length(metric) + 1) = metric || '{' AND substr(token, length(token)) = '}')
ON CONFLICT DO NOTHING
