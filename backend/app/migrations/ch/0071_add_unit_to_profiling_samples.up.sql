ALTER TABLE profiling_samples ADD COLUMN IF NOT EXISTS unit LowCardinality(String) DEFAULT ''
