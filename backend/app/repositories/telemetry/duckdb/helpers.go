//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
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

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func attrJSON(m map[string]string) (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// withAppender runs fn with an Appender on a dedicated connection — the
// Appender API is not reachable through database/sql, and Close flushes
// the appended rows.
func withAppender(ctx context.Context, table string, fn func(*duckdb.Appender)) error {
	conn, err := db.DuckDBConnector.Connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	appender, err := duckdb.NewAppenderFromConn(conn, "", table)
	if err != nil {
		return err
	}
	fn(appender)
	return appender.Close()
}
