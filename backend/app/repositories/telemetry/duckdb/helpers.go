//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"
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

// duckUUID converts to the driver's native UUID value: a zero-alloc
// [16]byte-to-[16]byte conversion, vs the 36-byte string uuid.String()
// allocates for VARCHAR columns.
func duckUUID(u uuid.UUID) duckdb.UUID {
	return duckdb.UUID(u)
}

func nullableUUID(u *uuid.UUID) any {
	if u == nil {
		return nil
	}
	return duckdb.UUID(*u)
}

const dropReportInterval = time.Minute

var (
	dropReportMu       sync.Mutex
	droppedSinceReport = map[string]uint64{}
	lastDropReportAt   = map[string]time.Time{}
)

// captureFlushFailure reports background-writer connect/flush failures with
// the same 1-minute-per-table rate limit as captureDroppedRow, keyed
// separately so a drop storm can't mask a flush failure.
func captureFlushFailure(table string, lostRows int, err error) {
	key := table + ":flush"

	var report bool
	dropReportMu.Lock()
	if time.Since(lastDropReportAt[key]) >= dropReportInterval {
		lastDropReportAt[key] = time.Now()
		report = true
	}
	dropReportMu.Unlock()

	if report {
		if lostRows > 0 {
			traceway.CaptureException(fmt.Errorf("duckdb %s background writer: %d acked rows lost, latest: %w", table, lostRows, err))
		} else {
			traceway.CaptureException(fmt.Errorf("duckdb %s background writer: connect/appender setup failing, no rows lost yet, latest: %w", table, err))
		}
	}
}

func captureDroppedRow(table string, err error) {
	db.RecordTelemetryRowDropped(table)

	var report uint64
	dropReportMu.Lock()
	droppedSinceReport[table]++
	if time.Since(lastDropReportAt[table]) >= dropReportInterval {
		report = droppedSinceReport[table]
		droppedSinceReport[table] = 0
		lastDropReportAt[table] = time.Now()
	}
	dropReportMu.Unlock()

	if report > 0 {
		traceway.CaptureException(fmt.Errorf("duckdb %s insert: dropped %d rows since last report, latest: %w", table, report, err))
	}
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
		db.RecordTelemetryInsertFailure()
		return err
	}
	defer conn.Close()

	appender, err := duckdb.NewAppenderFromConn(conn, "", table)
	if err != nil {
		db.RecordTelemetryInsertFailure()
		return err
	}
	fn(appender)
	if err := appender.Close(); err != nil {
		db.RecordTelemetryInsertFailure()
		return err
	}
	return nil
}
