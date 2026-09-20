//go:build !telemetry_ch && !telemetry_duckdb

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
)

type otelSpanRepository struct{}

var sqliteOtelCodec = shared.OtelValueCodec{
	UUID:   func(id uuid.UUID) any { return id.String() },
	Time:   func(t time.Time) any { return sqlitetypes.NewSQLiteTime(t) },
	Nanos:  func(n uint64) any { return strconv.FormatUint(n, 10) },
	Uint32: func(n uint32) any { return int64(n) },
}

func sqliteOtelValues(span models.OtelSpan, groups shared.OtelSpanGroups) ([]any, error) {
	row, err := shared.NewOtelSpanRow(span, groups)
	if err != nil {
		return nil, err
	}
	values, err := row.ScalarValues(sqliteOtelCodec)
	if err != nil {
		return nil, err
	}
	text, err := row.TextValues()
	if err != nil {
		return nil, err
	}
	return append(values, text...), nil
}

func (r *otelSpanRepository) InsertAsync(ctx context.Context, spans []models.OtelSpan) (int, error) {
	rejected, err := r.InsertWithRejections(ctx, spans)
	return len(rejected), err
}

func (r *otelSpanRepository) InsertWithRejections(ctx context.Context, spans []models.OtelSpan) ([]shared.SpanOwner, error) {
	if len(spans) == 0 {
		return nil, nil
	}
	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, shared.OtelTextInsertSQL)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	groups := shared.OtelSpanGroups{}
	var rejected []shared.SpanOwner
	for _, span := range spans {
		values, err := sqliteOtelValues(span, groups)
		if err != nil {
			shared.RecordRejectedOtelSpan(err)
			rejected = append(rejected, shared.SpanOwner{ProjectId: span.ProjectId, TraceId: span.TraceId, SpanId: span.SpanId})
			continue
		}
		if _, err := stmt.ExecContext(ctx, values...); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rejected, nil
}

var OtelSpanRepository = &otelSpanRepository{}

func scanSqliteTopology(rows *sql.Rows, withStatement bool) ([]models.OtelSpan, error) {
	result := make([]models.OtelSpan, 0)
	for rows.Next() {
		var span models.OtelSpan
		var nanos string
		columns := []any{&span.ProjectId, &span.TraceId, &span.SpanId, &span.ParentSpanId,
			&span.Name, &span.Duration, &span.SpanKind, &span.StatusCode, &span.ServiceName, &span.ScopeName, &nanos}
		if withStatement {
			columns = append(columns, &span.DbStatement)
		}
		if err := rows.Scan(columns...); err != nil {
			return nil, err
		}
		nanoValue, err := strconv.ParseUint(nanos, 10, 64)
		if err != nil {
			return nil, err
		}
		span.StartTime = shared.OtelNanosToTime(nanoValue)
		span.RecordedAt = span.StartTime
		result = append(result, span)
	}
	return result, rows.Err()
}

func (r *otelSpanRepository) FindTraceTopology(ctx context.Context, lookups []shared.SpanLookup, fromUnixNano uint64) ([]models.OtelSpan, error) {
	if len(lookups) == 0 {
		return nil, nil
	}
	query, args := shared.OtelTopologyQuery(lookups, false, fromUnixNano, sqliteOtelCodec.Time)
	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSqliteTopology(rows, false)
}

const sqliteStatementPreview = `substr(coalesce(json_extract(span_attributes, '$."db.query.text"'), json_extract(span_attributes, '$."db.statement"'), ''), 1, 240)`

func (r *otelSpanRepository) FindTraceOutline(ctx context.Context, lookups []shared.SpanLookup) ([]models.OtelSpan, error) {
	query, args := shared.OtelOutlineQuery(lookups, false, sqliteStatementPreview, sqliteOtelCodec.Time)
	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSqliteTopology(rows, true)
}

func (r *otelSpanRepository) FindSpanAttributes(ctx context.Context, lookup shared.SpanLookup, spanIds []string, limits shared.OtelAttributeLimits) (map[string]shared.OtelSpanAttributes, error) {
	args := []any{lookup.ProjectId.String(), lookup.TraceId}
	for _, id := range spanIds {
		args = append(args, id)
	}
	predicate := "project_id = ? AND trace_id = ? AND span_id IN (" + shared.OtelPlaceholders(len(spanIds)) + ")"
	predicate, args = shared.OtelWindowPredicate(predicate, args, []shared.SpanLookup{lookup}, sqliteOtelCodec.Time)
	rows, err := db.TelemetryDB.QueryContext(ctx, shared.OtelAttributeQuery("span_attributes", "length(CAST(span_attributes AS BLOB))", "CAST(start_time_unix_nano AS INTEGER)", predicate, shared.OtelTextWinnerOrder, limits), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return shared.ScanOtelAttributeRows(rows)
}

func (r *otelSpanRepository) IsReadLimitError(error) bool { return false }

var sqliteSearchDialect = shared.OtelSearchDialect{
	WinnerOrder: shared.OtelTextWinnerOrder,
	Project:     func(id uuid.UUID) any { return id.String() },
	Time:        sqliteOtelCodec.Time,
	Trace:       func(_ uuid.UUID, traceId string) (string, any) { return "trace_id = ?", traceId },
	NameLike:    "INSTR(LOWER(name), LOWER(?)) > 0",
	Attribute:   `json_extract(span_attributes, '$."' || ? || '"') = ?`,
	StartOrder:  "CAST(start_time_unix_nano AS INTEGER)",
}

func (r *otelSpanRepository) Search(ctx context.Context, search shared.OtelSpanSearch) ([]models.OtelSpan, uint64, error) {
	page, pageArgs, count, countArgs := shared.OtelSearchQueries(search, sqliteSearchDialect)
	var total uint64
	if err := db.TelemetryDB.QueryRowContext(ctx, count, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.TelemetryDB.QueryContext(ctx, page, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	spans, err := scanSqliteTopology(rows, false)
	return spans, total, err
}

func (r *otelSpanRepository) Services(ctx context.Context, project uuid.UUID, from, to time.Time) ([]string, error) {
	query, args := shared.OtelServicesQuery(project, from, to, sqliteSearchDialect)
	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	services := make([]string, 0)
	for rows.Next() {
		var service string
		if err := rows.Scan(&service); err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	return services, rows.Err()
}

func (r *otelSpanRepository) FindOTLP(ctx context.Context, project uuid.UUID, traceID, spanID string, at time.Time) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var value []byte
	from, to := shared.TraceWindowBounds(at.UTC())
	err := db.TelemetryDB.QueryRowContext(ctx, "SELECT otlp FROM "+shared.SpansTable+" WHERE project_id = ? AND trace_id = ? AND span_id = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY "+shared.OtelTextWinnerOrder+" LIMIT 1", project.String(), traceID, spanID, sqliteOtelCodec.Time(from), sqliteOtelCodec.Time(to)).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}
