//go:build transactional_pg && !telemetry_ch && !telemetry_firebolt

package db

// The undefined identifier makes this unsupported tag combination fail at compile time.
var _ = build_error__transactional_pg_requires_telemetry_ch_or_telemetry_firebolt__see_docs_learn_build_tags
