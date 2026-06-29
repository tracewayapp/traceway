//go:build duckdb && !pgch

package repositories

// The DuckDB Appender rejects typed Go pointers for nullable columns
// (cast error: cannot cast *string to string). It accepts an untyped nil
// for SQL NULL or the dereferenced value otherwise.
func nullableString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
