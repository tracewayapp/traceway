ALTER TABLE log_records ADD COLUMN IF NOT EXISTS session_id String MATERIALIZED log_attributes['session.id']
