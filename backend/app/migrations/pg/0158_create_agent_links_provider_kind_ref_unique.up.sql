CREATE UNIQUE INDEX IF NOT EXISTS agent_links_provider_kind_ref_unique ON agent_links (provider, kind, external_ref)
