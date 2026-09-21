//go:build !telemetry_ch && !telemetry_duckdb

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
)

// legacyRepository reads the tables V2 replaced, for the move-over. See shared/legacy.go for what the rows carry.
type legacyRepository struct{}

const legacyIdChunk = 500

const (
	legacyEndpointColumns = `id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name,
		COALESCE(distributed_trace_id, '') AS trace_id, COALESCE(span_id, '') AS span_id, '' AS parent_span_id, is_stream, is_root`
	legacyTaskColumns = `id, project_id, task_name, duration, recorded_at, client_ip, attributes, app_version, server_name,
		COALESCE(distributed_trace_id, '') AS trace_id, COALESCE(span_id, '') AS span_id, '' AS parent_span_id, is_root`
	legacyAiTraceColumns = `id, project_id, recorded_at, duration, status_code, model, response_model, provider, operation, input_tokens, output_tokens,
		total_tokens, cached_tokens, reasoning_tokens, input_cost, output_cost, total_cost, trace_name, user_id, finish_reason, server_name, app_version,
		storage_key, attributes, COALESCE(distributed_trace_id, '') AS trace_id, '' AS span_id, '' AS parent_span_id, is_root,
		conversation_id, tool_call_count, tool_names, flagged, flagged_terms`
	legacyExceptionColumns = `id, project_id, COALESCE(trace_id, '') AS trace_id, '' AS span_id, trace_type, exception_hash, stack_trace, recorded_at, attributes,
		app_version, server_name, is_message, COALESCE(distributed_trace_id, ''), session_id`
)

// legacyWindow sorts only overfull one-second slices; see shared.LegacyPageOrder.
func legacyWindow(columns, table string, from, to time.Time, limit int, offset *int) (string, lit.P) {
	if offset != nil {
		query, params := legacyWindow(columns, table, from, to, limit, nil)
		params["offset"] = *offset
		return strings.Replace(query, " LIMIT :limit", " ORDER BY "+shared.LegacyPageOrder(table, false)+" LIMIT :limit OFFSET :offset", 1), params
	}
	return "SELECT " + columns + " FROM " + table + " WHERE recorded_at >= :from AND recorded_at < :to LIMIT :limit",
		lit.P{"from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to), "limit": limit}
}

func legacyByIds(columns, table string, ids []uuid.UUID, from, to time.Time) (string, lit.P) {
	params := lit.P{"from": sqlitetypes.NewSQLiteTime(from), "to": sqlitetypes.NewSQLiteTime(to)}
	names := make([]string, len(ids))
	for i, id := range ids {
		key := fmt.Sprintf("id_%d", i)
		names[i], params[key] = ":"+key, id
	}
	return "SELECT " + columns + " FROM " + table + " WHERE id IN (" + strings.Join(names, ",") + ") AND recorded_at >= :from AND recorded_at < :to", params
}

func legacyEndpoints(ctx context.Context, query string, params lit.P) ([]models.Endpoint, error) {
	rows, err := shared.SelectLegacy[endpoint](ctx, db.TelemetryDB, query, params)
	if err != nil {
		return nil, err
	}
	found := make([]models.Endpoint, len(rows))
	for i, row := range rows {
		found[i] = row.toModel()
	}
	return found, nil
}

func legacyTasks(ctx context.Context, query string, params lit.P) ([]models.Task, error) {
	rows, err := shared.SelectLegacy[task](ctx, db.TelemetryDB, query, params)
	if err != nil {
		return nil, err
	}
	found := make([]models.Task, len(rows))
	for i, row := range rows {
		found[i] = row.toModel()
	}
	return found, nil
}

func legacyAiTraces(ctx context.Context, query string, params lit.P) ([]models.AiTrace, error) {
	rows, err := shared.SelectLegacy[aiTraceRow](ctx, db.TelemetryDB, query, params)
	if err != nil {
		return nil, err
	}
	found := make([]models.AiTrace, len(rows))
	for i, row := range rows {
		found[i] = row.toModel()
	}
	return found, nil
}

func (legacyRepository) FindEndpoints(ctx context.Context, from, to time.Time, limit int, offset *int) ([]models.Endpoint, error) {
	query, params := legacyWindow(legacyEndpointColumns, "endpoints", from, to, limit, offset)
	return legacyEndpoints(ctx, query, params)
}

func (legacyRepository) FindTasks(ctx context.Context, from, to time.Time, limit int, offset *int) ([]models.Task, error) {
	query, params := legacyWindow(legacyTaskColumns, "tasks", from, to, limit, offset)
	return legacyTasks(ctx, query, params)
}

func (legacyRepository) FindAiTraces(ctx context.Context, from, to time.Time, limit int, offset *int) ([]models.AiTrace, error) {
	query, params := legacyWindow(legacyAiTraceColumns, "ai_traces", from, to, limit, offset)
	return legacyAiTraces(ctx, query, params)
}

func (legacyRepository) FindExceptions(ctx context.Context, from, to time.Time, limit int, offset *int) ([]shared.LegacyException, error) {
	query, params := legacyWindow(legacyExceptionColumns, "exception_stack_traces", from, to, limit, offset)
	query, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return nil, err
	}
	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var found []shared.LegacyException
	for rows.Next() {
		var row exceptionRow
		var traceId string
		if err := rows.Scan(&row.Id, &row.ProjectId, &row.TraceId, &row.SpanId, &row.TraceType, &row.ExceptionHash, &row.StackTrace, &row.RecordedAt, &row.Attributes, &row.AppVersion, &row.ServerName, &row.IsMessage, &traceId, &row.SessionId); err != nil {
			return nil, err
		}
		found = append(found, shared.LegacyException{ExceptionStackTrace: row.toModel(), DistributedTraceId: traceId})
	}
	return found, rows.Err()
}

func (legacyRepository) FindSpans(ctx context.Context, from, to time.Time, limit int, offset *int) ([]shared.LegacySpan, error) {
	query, params := legacyWindow("project_id, id, trace_id, COALESCE(parent_span_id, ''), name, start_time, duration, recorded_at, attributes", "spans", from, to, limit, offset)
	query, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return nil, err
	}
	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var found []shared.LegacySpan
	for rows.Next() {
		var span shared.LegacySpan
		var start, recorded sqlitetypes.SQLiteTime
		var attributes sqlitetypes.SQLiteJSONMap
		var duration int64
		if err := rows.Scan(&span.ProjectId, &span.Id, &span.OwnerId, &span.ParentSpanId, &span.Name, &start, &duration, &recorded, &attributes); err != nil {
			return nil, err
		}
		span.StartTime, span.RecordedAt, span.Duration, span.Attributes = start.Time, recorded.Time, time.Duration(duration), attributes
		found = append(found, span)
	}
	return found, rows.Err()
}

// FindOwners looks the owners of spans and exceptions up by id, in all three tables an owner can live in.
func (legacyRepository) FindOwners(ctx context.Context, ids []uuid.UUID, from, to time.Time) ([]models.Endpoint, []models.Task, []models.AiTrace, error) {
	var endpoints []models.Endpoint
	var tasks []models.Task
	var aiTraces []models.AiTrace
	for chunk := range slices.Chunk(ids, legacyIdChunk) {
		query, params := legacyByIds(legacyEndpointColumns, "endpoints", chunk, from, to)
		foundEndpoints, err := legacyEndpoints(ctx, query, params)
		if err != nil {
			return nil, nil, nil, err
		}
		query, params = legacyByIds(legacyTaskColumns, "tasks", chunk, from, to)
		foundTasks, err := legacyTasks(ctx, query, params)
		if err != nil {
			return nil, nil, nil, err
		}
		query, params = legacyByIds(legacyAiTraceColumns, "ai_traces", chunk, from, to)
		foundAiTraces, err := legacyAiTraces(ctx, query, params)
		if err != nil {
			return nil, nil, nil, err
		}
		endpoints, tasks, aiTraces = append(endpoints, foundEndpoints...), append(tasks, foundTasks...), append(aiTraces, foundAiTraces...)
	}
	return endpoints, tasks, aiTraces, nil
}

func (legacyRepository) FindBounds(ctx context.Context, table string) (oldest, newest time.Time, found bool, err error) {
	if table, err = shared.LegacyTable(table); err != nil {
		return oldest, newest, false, err
	}
	var low, high sqlitetypes.SQLiteTime
	var count int64
	if err = db.TelemetryDB.QueryRowContext(ctx, "SELECT COUNT(*), MIN(recorded_at), MAX(recorded_at) FROM "+table).Scan(&count, &low, &high); err != nil {
		return oldest, newest, false, err
	}
	return low.Time, high.Time, count > 0, nil
}

// FindMovedIds returns full occurrence keys, never IDs shared by unrelated traces.
func (legacyRepository) FindMovedIds(ctx context.Context, table string, ids []uuid.UUID, from, to time.Time) (map[shared.MovedOccurrence]bool, error) {
	table, err := shared.LegacyTable(table)
	if err != nil {
		return nil, err
	}
	moved := map[shared.MovedOccurrence]bool{}
	for chunk := range slices.Chunk(ids, legacyIdChunk) {
		args := []any{sqlitetypes.NewSQLiteTime(from), sqlitetypes.NewSQLiteTime(to)}
		for _, id := range chunk {
			args = append(args, id)
		}
		rows, err := db.TelemetryDB.QueryContext(ctx, "SELECT project_id, id, trace_id, span_id, recorded_at FROM "+table+" WHERE recorded_at >= ? AND recorded_at < ? AND id IN (?"+strings.Repeat(",?", len(chunk)-1)+")", args...)
		if err != nil {
			return nil, err
		}
		if err := scanMovedIds(rows, moved); err != nil {
			return nil, err
		}
	}
	return moved, nil
}

func scanMovedIds(rows *sql.Rows, moved map[shared.MovedOccurrence]bool) error {
	defer rows.Close()
	for rows.Next() {
		var project, id uuid.UUID
		var trace, span string
		var at sqlitetypes.SQLiteTime
		if err := rows.Scan(&project, &id, &trace, &span, &at); err != nil {
			return err
		}
		moved[shared.MovedOccurrenceKey(project, id, trace, span, at.Time)] = true
	}
	return rows.Err()
}

func (legacyRepository) FindMovedSpans(ctx context.Context, traceIds []string, from, to time.Time) (map[string]bool, error) {
	moved := map[string]bool{}
	for chunk := range slices.Chunk(traceIds, legacyIdChunk) {
		args := []any{sqlitetypes.NewSQLiteTime(from), sqlitetypes.NewSQLiteTime(to)}
		for _, id := range chunk {
			args = append(args, id)
		}
		rows, err := db.TelemetryDB.QueryContext(ctx, "SELECT project_id, trace_id, span_id FROM "+shared.SpansTable+" WHERE recorded_at >= ? AND recorded_at < ? AND trace_id IN (?"+strings.Repeat(",?", len(chunk)-1)+")", args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var project uuid.UUID
			var trace, span string
			if err := rows.Scan(&project, &trace, &span); err != nil {
				rows.Close()
				return nil, err
			}
			moved[shared.MovedSpanKey(project, trace, span)] = true
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return moved, nil
}

// OccurrenceTime matches the destination column precision for recovery keys.
func (legacyRepository) OccurrenceTime(table string, at time.Time) time.Time {
	return at
}

var LegacyRepository = legacyRepository{}
