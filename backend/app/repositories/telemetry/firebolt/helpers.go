//go:build telemetry_firebolt

package firebolt

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
)

// Firebolt takes `?` placeholders, so telemetry queries render with the
// SQLite dialect — never db.Driver, which is lit.PostgreSQL in this build.
var litDriver = lit.SQLite

func registerModels(register func(driver lit.Driver)) {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(lit.Driver) {
		register(litDriver)
	})
}

func parseNamed(query string, params lit.P) (string, []any, error) {
	return lit.ParseNamedQuery(litDriver, query, params)
}

// sqlitetypes registers these with the main-DB driver; lit's registry is
// last-write-wins per type and this init runs later, so this rebinds them
// to the `?` dialect. Nothing outside telemetry uses them.
func init() {
	registerModels(func(driver lit.Driver) {
		lit.RegisterModel[sqlitetypes.TimeSeriesResult](driver)
		lit.RegisterModel[sqlitetypes.GroupedTimeSeriesResult](driver)
		lit.RegisterModel[sqlitetypes.FilePathResult](driver)
	})
}

// insertRows bulk-inserts chunked multi-row INSERTs rendered as SQL
// literals — substantially faster than driver placeholder substitution on
// large batches. Firebolt string literals treat backslashes literally, so
// doubling single quotes is the only escaping needed. A failed statement
// fails the whole call (the request 500s and the SDK retries the frame).
const insertChunkRows = 500

func renderLiteral(sb *strings.Builder, v any) error {
	switch x := v.(type) {
	case nil:
		sb.WriteString("NULL")
	case string:
		sb.WriteByte('\'')
		sb.WriteString(strings.ReplaceAll(x, "'", "''"))
		sb.WriteByte('\'')
	case int64:
		sb.WriteString(strconv.FormatInt(x, 10))
	case int:
		sb.WriteString(strconv.Itoa(x))
	case uint32:
		sb.WriteString(strconv.FormatUint(uint64(x), 10))
	case float64:
		sb.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
	case bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case time.Time:
		sb.WriteByte('\'')
		sb.WriteString(x.UTC().Format("2006-01-02 15:04:05.000000"))
		sb.WriteByte('\'')
	default:
		return fmt.Errorf("unsupported literal type %T", v)
	}
	return nil
}

func insertRows(ctx context.Context, table string, columns []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	if b := activeBatcher(); b != nil && b.enqueue(table, columns, rows) {
		return nil
	}
	return insertRowsDirect(ctx, table, columns, rows)
}

func insertRowsDirect(ctx context.Context, table string, columns []string, rows [][]any) error {
	if _, _, ok := copyDirs(); ok && len(rows) >= copyMinRows {
		if err := copyIngest(ctx, table, columns, rows); err == nil {
			return nil
		}
		// fall through to the literal path — copyIngest is an optimization
	}
	prefix := "INSERT INTO " + table + " (" + strings.Join(columns, ", ") + ") VALUES "

	for start := 0; start < len(rows); start += insertChunkRows {
		end := start + insertChunkRows
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]

		var sb strings.Builder
		sb.Grow(len(prefix) + len(chunk)*len(columns)*24)
		sb.WriteString(prefix)
		for i, row := range chunk {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteByte('(')
			for j, v := range row {
				if j > 0 {
					sb.WriteByte(',')
				}
				if err := renderLiteral(&sb, v); err != nil {
					db.RecordTelemetryInsertFailure()
					return fmt.Errorf("firebolt %s insert: %w", table, err)
				}
			}
			sb.WriteByte(')')
		}

		if _, err := db.TelemetryDB.ExecContext(ctx, sb.String()); err != nil {
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

// The aggregating indexes key time by DATE_TRUNC('minute', col). A query
// merges index states only when its time filter and buckets are expressed
// over that exact key expression — any raw-column reference forces a full
// scan. The minute snap moves window edges by <60s, below dashboard
// resolution. Bind :from_min/:to_min via bindMinuteRange.
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
