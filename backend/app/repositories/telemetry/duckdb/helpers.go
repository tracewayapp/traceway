//go:build telemetry_duckdb

package duckdb

import (
	"fmt"

	traceway "go.tracewayapp.com"
)

// The DuckDB Appender rejects typed Go pointers for nullable columns
// (cast error: cannot cast *string to string). It accepts an untyped nil
// for SQL NULL or the dereferenced value otherwise.
func nullableString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func captureDroppedRow(table string, err error) {
	traceway.CaptureException(fmt.Errorf("duckdb %s insert: dropping row: %w", table, err))
}

func timeBucketExpr(column string, intervalSeconds int) string {
	return fmt.Sprintf("time_bucket(to_seconds(%d), %s, TIMESTAMP '1970-01-01')", intervalSeconds, column)
}
