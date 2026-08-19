CREATE UNIQUE INDEX IF NOT EXISTS post_mortems_incident_unique ON post_mortems (incident_id) WHERE incident_id IS NOT NULL
