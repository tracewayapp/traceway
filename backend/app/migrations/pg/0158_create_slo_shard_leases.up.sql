CREATE TABLE IF NOT EXISTS slo_shard_leases (
    shard INTEGER PRIMARY KEY,
    owner TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
)
