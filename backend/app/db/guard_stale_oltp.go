//go:build oltp_pg || oltp_sqlite

package db

// The oltp_* axis was renamed; the undefined identifier fails the build so a
// stale invocation cannot silently produce the dual-SQLite binary.
var _ = build_error__oltp_tags_were_renamed__use_transactional_pg_and_transactional_sqlite
