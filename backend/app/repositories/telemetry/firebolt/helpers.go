//go:build telemetry_firebolt

package firebolt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
)

// The Firebolt engine accepts `?` placeholders (the Go SDK substitutes them
// client-side), so this package renders every lit named query with the SQLite
// driver regardless of db.Driver — which is lit.PostgreSQL in this build,
// since the main DB is Postgres. Model registration pins the same driver so
// SelectNamed/SelectSingleNamed resolve to `?` rendering.
var litDriver = lit.SQLite

// registerModels queues row-model registrations pinned to the Firebolt
// placeholder dialect, ignoring the main-DB driver models.Init passes in.
func registerModels(register func(driver lit.Driver)) {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(lit.Driver) {
		register(litDriver)
	})
}

func parseNamed(query string, params lit.P) (string, []any, error) {
	return lit.ParseNamedQuery(litDriver, query, params)
}

// sqlitetypes registers its shared result models with the driver models.Init
// passes in — lit.PostgreSQL in this build. lit's registry is last-write-wins
// per type, and this package's registrations run after sqlitetypes' (package
// init order), so re-registering here rebinds them to the `?` dialect every
// telemetry query needs. Nothing outside telemetry uses these types.
func init() {
	registerModels(func(driver lit.Driver) {
		lit.RegisterModel[sqlitetypes.TimeSeriesResult](driver)
		lit.RegisterModel[sqlitetypes.GroupedTimeSeriesResult](driver)
		lit.RegisterModel[sqlitetypes.FilePathResult](driver)
	})
}

// insertRows bulk-inserts rows as chunked multi-row INSERT statements.
// A failed statement fails the whole call (the request 500s and the SDK
// retries the frame), matching the SQLite/ClickHouse backends' behavior.
const insertChunkRows = 500

func insertRows(ctx context.Context, table string, columns []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	rowPlaceholders := "(" + strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",") + ")"
	prefix := "INSERT INTO " + table + " (" + strings.Join(columns, ", ") + ") VALUES "

	for start := 0; start < len(rows); start += insertChunkRows {
		end := start + insertChunkRows
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]

		var sb strings.Builder
		sb.Grow(len(prefix) + len(chunk)*(len(rowPlaceholders)+1))
		sb.WriteString(prefix)
		args := make([]any, 0, len(chunk)*len(columns))
		for i, row := range chunk {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(rowPlaceholders)
			args = append(args, row...)
		}

		if _, err := db.TelemetryDB.ExecContext(ctx, sb.String(), args...); err != nil {
			db.RecordTelemetryInsertFailure()
			return fmt.Errorf("firebolt %s insert: %w", table, err)
		}
	}
	return nil
}

// timeBucketExpr floors a timestamp column to fixed-width buckets anchored at
// the epoch, matching the SQLite and DuckDB backends' chart bucketing.
func timeBucketExpr(column string, intervalSeconds int) string {
	return fmt.Sprintf("TO_TIMESTAMP(FLOOR(EXTRACT(EPOCH FROM %s) / %d) * %d)", column, intervalSeconds, intervalSeconds)
}

// The aggregating indexes key time by DATE_TRUNC('minute', col). A query is
// rewritten to merge index states only when its time filter and buckets are
// expressed over that exact key expression — any use of the raw column forces
// a full scan. indexBucketExpr therefore buckets over the minute key (charts
// never use sub-minute intervals), and indexMinuteRange emits the filter
// fragment whose :from_min/:to_min bounds must be bound via bindMinuteRange.
// The minute snap moves each window edge by <60s, below dashboard resolution.
func indexBucketExpr(column string, intervalSeconds int) string {
	if intervalSeconds <= 60 {
		return fmt.Sprintf("DATE_TRUNC('minute', %s)", column)
	}
	return fmt.Sprintf("TO_TIMESTAMP(FLOOR(EXTRACT(EPOCH FROM DATE_TRUNC('minute', %s)) / %d) * %d)", column, intervalSeconds, intervalSeconds)
}

func indexMinuteRange(column string) string {
	return fmt.Sprintf("DATE_TRUNC('minute', %s) >= :from_min AND DATE_TRUNC('minute', %s) <= :to_min", column, column)
}

func bindMinuteRange(params lit.P, from, to time.Time) {
	params["from_min"] = from.UTC().Truncate(time.Minute)
	params["to_min"] = to.UTC().Truncate(time.Minute)
}

func minuteRangeParams(projectId uuid.UUID, from, to time.Time) lit.P {
	params := lit.P{"project_id": projectId}
	bindMinuteRange(params, from, to)
	return params
}

// jsonExtractExpr extracts a top-level string key from a JSON TEXT column.
// Firebolt uses JSON pointers; '~' and '/' in keys are escaped per RFC 6901.
func jsonExtractExpr(column string, keyExprSQL string) string {
	return "JSON_POINTER_EXTRACT_TEXT(" + column + ", '/' || " + keyExprSQL + ")"
}

func jsonPointerEscape(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
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

func nullableString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
