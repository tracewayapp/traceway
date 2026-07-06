//go:build (telemetry_sqlite && (telemetry_ch || telemetry_duckdb)) || (oltp_sqlite && oltp_pg)

package db

// telemetry_sqlite and oltp_sqlite are no-op tags (SQLite is the default); the
// undefined identifier rejects passing them together with a conflicting backend.
var _ = build_error__conflicting_backend_tags__sqlite_is_the_default_when_no_tag_is_set
