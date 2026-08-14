ALTER TABLE check_results MODIFY TTL toDateTime(recorded_at) + INTERVAL 90 DAY
