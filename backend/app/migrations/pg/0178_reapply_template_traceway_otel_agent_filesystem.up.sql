-- Databases upgraded from flutter-sdk-v2 used 0175 for the Flutter template
-- and skipped main's correction to the agent template at the same version.
UPDATE dashboard_templates
SET definition = jsonb_set(definition, '{widgets,6,config,sources,0,tagFilters}', '{"mode":"rw"}'),
    updated_at = NOW()
WHERE key = 'traceway-otel-agent'
  AND definition #>> '{widgets,6,config,sources,0,name}' = 'system.filesystem.utilization'
  AND definition #> '{widgets,6,config,sources,0,tagFilters}' = '{"state":"used"}'::jsonb
