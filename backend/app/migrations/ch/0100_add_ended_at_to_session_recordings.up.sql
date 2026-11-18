ALTER TABLE session_recordings ADD COLUMN IF NOT EXISTS ended_at Nullable(DateTime64(3)) DEFAULT NULL
