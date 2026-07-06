//go:build duckdb

package db

// The duckdb tag was renamed; the undefined identifier fails the build so a stale
// invocation cannot silently produce the dual-SQLite binary.
var _ = build_error__tag_duckdb_was_renamed__use_telemetry_duckdb
