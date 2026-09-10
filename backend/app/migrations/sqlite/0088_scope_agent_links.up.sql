DROP INDEX IF EXISTS agent_links_provider_kind_ref_unique;
CREATE UNIQUE INDEX agent_links_attempt_artifact_unique ON agent_links (attempt_id, COALESCE(integration_id, 0), provider, kind, external_ref);
