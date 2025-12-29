CREATE TABLE demos
(
    `device_id` UInt32,
    `organization_id` UInt32,
    `safety_trip_id` UUID,
    `event_type` UInt8,
    `event_time` DateTime64(3, 'UTC'),
    `pos_lat` Float64,
    `pos_long` Float64,
    `odometer` Nullable(Float64),
    `velocity` Nullable(Float32),
    `location` String,
    `total_miles` Nullable(Float32),
    `total_score` Nullable(UInt32)
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(event_time)
ORDER BY (organization_id, device_id, safety_trip_id, event_time)
SETTINGS index_granularity = 8192