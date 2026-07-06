//go:build pgch

package db

// The pgch tag was split into two axes; the undefined identifier fails the build
// so a stale invocation cannot silently produce the dual-SQLite binary.
var _ = build_error__tag_pgch_was_renamed__use_oltp_pg_and_telemetry_ch
