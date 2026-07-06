//go:build telemetry_ch && !oltp_pg

package db

// The undefined identifier makes this unsupported tag combination fail at compile time.
var _ = build_error__telemetry_ch_requires_oltp_pg__see_docs_learn_build_tags
