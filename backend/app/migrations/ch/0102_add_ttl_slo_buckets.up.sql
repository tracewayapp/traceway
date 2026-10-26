ALTER TABLE slo_buckets MODIFY TTL toDateTime(bucket_start) + INTERVAL 120 DAY
