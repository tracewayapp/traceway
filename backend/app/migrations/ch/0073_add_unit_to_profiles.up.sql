ALTER TABLE profiles ADD COLUMN IF NOT EXISTS unit LowCardinality(String) DEFAULT ''
