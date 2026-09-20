//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"time"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

type otelSpanRepository struct{}

var duckdbOtelCodec = shared.OtelValueCodec{
	UUID:   func(id uuid.UUID) any { return id.String() },
	Time:   func(t time.Time) any { return t.UTC() },
	Nanos:  func(n uint64) any { return n },
	Uint32: func(n uint32) any { return int64(n) },
}

func duckdbOtelValues(span models.OtelSpan, groups shared.OtelSpanGroups) ([]driver.Value, error) {
	row, err := shared.NewOtelSpanRow(span, groups)
	if err != nil {
		return nil, err
	}
	values, err := row.ScalarValues(duckdbOtelCodec)
	if err != nil {
		return nil, err
	}
	text, err := row.TextValues()
	if err != nil {
		return nil, err
	}
	values = append(append(values, text...), shared.OtelKey(span.ProjectId, span.TraceId))
	driverValues := make([]driver.Value, len(values))
	for i, value := range values {
		driverValues[i] = value
	}
	return driverValues, nil
}

func (r *otelSpanRepository) InsertAsync(ctx context.Context, spans []models.OtelSpan) (int, error) {
	if len(spans) == 0 {
		return 0, nil
	}
	rejected := 0
	err := withAppenderColumns(ctx, shared.SpansTable, strings.Split(shared.OtelTextStorageColumns+", trace_key", ", "), func(appender *duckdb.Appender) {
		groups := shared.OtelSpanGroups{}
		for _, span := range spans {
			values, err := duckdbOtelValues(span, groups)
			if err == nil {
				err = appender.AppendRow(values...)
			}
			if err != nil {
				captureDroppedRow(shared.SpansTable, err)
				rejected++
			}
		}
	})
	if err != nil {
		return 0, err
	}
	return rejected, nil
}

var OtelSpanRepository = &otelSpanRepository{}

func scanDuckdbTopology(rows *sql.Rows, withStatement bool) ([]models.OtelSpan, error) {
	result := make([]models.OtelSpan, 0)
	for rows.Next() {
		var span models.OtelSpan
		var nanos uint64
		columns := []any{&span.ProjectId, &span.TraceId, &span.SpanId, &span.ParentSpanId,
			&span.Name, &span.Duration, &span.SpanKind, &span.StatusCode, &span.ServiceName, &span.ScopeName, &nanos}
		if withStatement {
			columns = append(columns, &span.DbStatement)
		}
		if err := rows.Scan(columns...); err != nil {
			return nil, err
		}
		span.StartTime = shared.OtelNanosToTime(nanos)
		span.RecordedAt = span.StartTime
		result = append(result, span)
	}
	return result, rows.Err()
}

func (r *otelSpanRepository) FindTraceTopology(ctx context.Context, lookups []shared.SpanLookup, fromUnixNano uint64) ([]models.OtelSpan, error) {
	if len(lookups) == 0 {
		return nil, nil
	}
	query, args := shared.OtelTopologyQuery(lookups, true, fromUnixNano, duckdbOtelCodec.Time)
	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDuckdbTopology(rows, false)
}

const duckdbStatementPreview = `substr(coalesce(json_extract_string(span_attributes, '$."db.query.text"'), json_extract_string(span_attributes, '$."db.statement"'), ''), 1, 240)`

func (r *otelSpanRepository) FindTraceOutline(ctx context.Context, lookups []shared.SpanLookup) ([]models.OtelSpan, error) {
	query, args := shared.OtelOutlineQuery(lookups, true, duckdbStatementPreview, duckdbOtelCodec.Time)
	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDuckdbTopology(rows, true)
}

func (r *otelSpanRepository) FindSpanAttributes(ctx context.Context, lookup shared.SpanLookup, spanIds []string, limits shared.OtelAttributeLimits) (map[string]shared.OtelSpanAttributes, error) {
	args := []any{shared.OtelKey(lookup.ProjectId, lookup.TraceId)}
	for _, id := range spanIds {
		args = append(args, id)
	}
	predicate := "trace_key = ? AND span_id IN (" + shared.OtelPlaceholders(len(spanIds)) + ")"
	predicate, args = shared.OtelWindowPredicate(predicate, args, []shared.SpanLookup{lookup}, duckdbOtelCodec.Time)
	rows, err := db.TelemetryDB.QueryContext(ctx, shared.OtelAttributeQuery("span_attributes", "start_time_unix_nano", predicate, limits), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return shared.ScanOtelAttributeRows(rows)
}

func (r *otelSpanRepository) IsReadLimitError(error) bool { return false }

var duckdbSearchDialect = shared.OtelSearchDialect{
	Project: func(id uuid.UUID) any { return id.String() },
	Time:    duckdbOtelCodec.Time,
	Trace: func(project uuid.UUID, traceHex string) (string, any) {
		return "trace_key = ?", shared.OtelKey(project, traceHex)
	},
	NameLike:   "contains(lower(name), lower(?))",
	Attribute:  `json_extract_string(span_attributes, '$."' || ? || '"') = ?`,
	StartOrder: "start_time_unix_nano",
}

func (r *otelSpanRepository) Search(ctx context.Context, search shared.OtelSpanSearch) ([]models.OtelSpan, uint64, error) {
	page, pageArgs, count, countArgs := shared.OtelSearchQueries(search, duckdbSearchDialect)
	var total uint64
	if err := db.TelemetryDB.QueryRowContext(ctx, count, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.TelemetryDB.QueryContext(ctx, page, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	spans, err := scanDuckdbTopology(rows, false)
	return spans, total, err
}

func (r *otelSpanRepository) Services(ctx context.Context, project uuid.UUID, from, to time.Time) ([]string, error) {
	query, args := shared.OtelServicesQuery(project, from, to, duckdbSearchDialect)
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
	err := db.TelemetryDB.QueryRowContext(ctx, "SELECT otlp FROM "+shared.SpansTable+" WHERE trace_key = ? AND span_id = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY duration DESC, otlp DESC LIMIT 1", shared.OtelKey(project, traceID), spanID, from.UTC(), to.UTC()).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}
