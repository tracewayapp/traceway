//go:build telemetry_firebolt && !transactional_pg

package db

// The undefined identifier makes this unsupported tag combination fail at compile time.
var _ = build_error__telemetry_firebolt_requires_transactional_pg__see_docs_learn_build_tags
