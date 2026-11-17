ALTER TABLE spans_v2 ADD COLUMN IF NOT EXISTS session_id String MATERIALIZED span_attributes['session.id']
