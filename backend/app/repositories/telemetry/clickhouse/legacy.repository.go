//go:build telemetry_ch

package clickhouse

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

// legacyRepository reads the tables V2 replaced, for the move-over. See shared/legacy.go for what the rows carry.
type legacyRepository struct{}

const legacyIdChunk = 1000

const (
	legacyEndpointColumns = `id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name,
		ifNull(toString(distributed_trace_id), ''), ifNull(toString(span_id), ''), is_stream, is_root`
	legacyTaskColumns = `id, project_id, task_name, duration, recorded_at, client_ip, attributes, app_version, server_name,
		ifNull(toString(distributed_trace_id), ''), ifNull(toString(span_id), ''), is_root`
	legacyAiTraceColumns = `id, project_id, recorded_at, duration, status_code, model, response_model, provider, operation, input_tokens, output_tokens,
		total_tokens, cached_tokens, reasoning_tokens, input_cost, output_cost, total_cost, trace_name, user_id, finish_reason, server_name, app_version,
		storage_key, attributes, ifNull(toString(distributed_trace_id), ''), '', '', is_root, conversation_id, tool_call_count, tool_names, flagged, flagged_terms`
)

func legacyAttributes(raw string) map[string]string {
	var attributes map[string]string
	if raw != "" && raw != "{}" && json.Unmarshal([]byte(raw), &attributes) != nil {
		return nil
	}
	return attributes
}

// legacyWindow sorts only overfull one-second slices; see shared.LegacyPageOrder.
func legacyWindow(columns, table string, from, to time.Time, limit int, offset *int) (string, []any) {
	if offset != nil {
		return "SELECT " + columns + " FROM " + table + " WHERE recorded_at >= ? AND recorded_at < ? ORDER BY " + shared.LegacyPageOrder(table, true) + " LIMIT ? OFFSET ?", []any{from, to, limit, *offset}
	}
	return "SELECT " + columns + " FROM " + table + " WHERE recorded_at >= ? AND recorded_at < ? LIMIT ?", []any{from, to, limit}
}

func legacyByIds(columns, table string, ids []uuid.UUID, from, to time.Time) (string, []any) {
	return "SELECT " + columns + " FROM " + table + " WHERE id IN (?) AND recorded_at >= ? AND recorded_at < ?", []any{ids, from, to}
}

func legacyRows[T any](ctx context.Context, query string, args []any, scan func(driver.Rows) (T, error)) ([]T, error) {
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var found []T
	for rows.Next() {
		row, err := scan(rows)
		if err != nil {
			return nil, err
		}
		found = append(found, row)
	}
	return found, rows.Err()
}

func scanLegacyEndpoint(rows driver.Rows) (models.Endpoint, error) {
	var t models.Endpoint
	var attributes string
	var isStream, isRoot uint8
	err := rows.Scan(&t.Id, &t.ProjectId, &t.Endpoint, &t.Duration, &t.RecordedAt, &t.StatusCode, &t.BodySize, &t.ClientIP, &attributes, &t.AppVersion,
		&t.ServerName, &t.TraceId, &t.SpanId, &isStream, &isRoot)
	t.Attributes, t.IsStream, t.IsRoot = legacyAttributes(attributes), isStream == 1, isRoot == 1
	return t, err
}

func scanLegacyTask(rows driver.Rows) (models.Task, error) {
	var t models.Task
	var attributes string
	var isRoot uint8
	err := rows.Scan(&t.Id, &t.ProjectId, &t.TaskName, &t.Duration, &t.RecordedAt, &t.ClientIP, &attributes, &t.AppVersion, &t.ServerName, &t.TraceId, &t.SpanId, &isRoot)
	t.Attributes, t.IsRoot = legacyAttributes(attributes), isRoot == 1
	return t, err
}

func scanLegacyAiTrace(rows driver.Rows) (models.AiTrace, error) {
	return scanAiTrace(rows.Scan)
}

func (legacyRepository) FindEndpoints(ctx context.Context, from, to time.Time, limit int, offset *int) ([]models.Endpoint, error) {
	query, args := legacyWindow(legacyEndpointColumns, "endpoints", from, to, limit, offset)
	return legacyRows(ctx, query, args, scanLegacyEndpoint)
}

func (legacyRepository) FindTasks(ctx context.Context, from, to time.Time, limit int, offset *int) ([]models.Task, error) {
	query, args := legacyWindow(legacyTaskColumns, "tasks", from, to, limit, offset)
	return legacyRows(ctx, query, args, scanLegacyTask)
}

func (legacyRepository) FindAiTraces(ctx context.Context, from, to time.Time, limit int, offset *int) ([]models.AiTrace, error) {
	query, args := legacyWindow(legacyAiTraceColumns, "ai_traces", from, to, limit, offset)
	return legacyRows(ctx, query, args, scanLegacyAiTrace)
}

func (legacyRepository) FindExceptions(ctx context.Context, from, to time.Time, limit int, offset *int) ([]shared.LegacyException, error) {
	query, args := legacyWindow(`id, project_id, ifNull(toString(trace_id), ''), trace_type, exception_hash, stack_trace, recorded_at, attributes, app_version,
		server_name, is_message, ifNull(toString(distributed_trace_id), ''), session_id`, "exception_stack_traces", from, to, limit, offset)
	return legacyRows(ctx, query, args, func(rows driver.Rows) (shared.LegacyException, error) {
		var est shared.LegacyException
		var attributes string
		var isMessage uint8
		err := rows.Scan(&est.Id, &est.ProjectId, &est.TraceId, &est.TraceType, &est.ExceptionHash, &est.StackTrace, &est.RecordedAt, &attributes,
			&est.AppVersion, &est.ServerName, &isMessage, &est.DistributedTraceId, &est.SessionId)
		est.Attributes, est.IsMessage = legacyAttributes(attributes), isMessage == 1
		return est, err
	})
}

func (legacyRepository) FindSpans(ctx context.Context, from, to time.Time, limit int, offset *int) ([]shared.LegacySpan, error) {
	query, args := legacyWindow(`project_id, id, trace_id, ifNull(toString(parent_span_id), ''), name, start_time, duration, recorded_at, attributes`, "spans", from, to, limit, offset)
	return legacyRows(ctx, query, args, func(rows driver.Rows) (shared.LegacySpan, error) {
		var span shared.LegacySpan
		var attributes string
		var duration int64
		err := rows.Scan(&span.ProjectId, &span.Id, &span.OwnerId, &span.ParentSpanId, &span.Name, &span.StartTime, &duration, &span.RecordedAt, &attributes)
		span.Duration, span.Attributes = time.Duration(duration), legacyAttributes(attributes)
		return span, err
	})
}

// FindOwners looks the owners of spans and exceptions up by id, in all three tables an owner can live in.
func (legacyRepository) FindOwners(ctx context.Context, ids []uuid.UUID, from, to time.Time) ([]models.Endpoint, []models.Task, []models.AiTrace, error) {
	var endpoints []models.Endpoint
	var tasks []models.Task
	var aiTraces []models.AiTrace
	for chunk := range slices.Chunk(ids, legacyIdChunk) {
		query, args := legacyByIds(legacyEndpointColumns, "endpoints", chunk, from, to)
		foundEndpoints, err := legacyRows(ctx, query, args, scanLegacyEndpoint)
		if err != nil {
			return nil, nil, nil, err
		}
		query, args = legacyByIds(legacyTaskColumns, "tasks", chunk, from, to)
		foundTasks, err := legacyRows(ctx, query, args, scanLegacyTask)
		if err != nil {
			return nil, nil, nil, err
		}
		query, args = legacyByIds(legacyAiTraceColumns, "ai_traces", chunk, from, to)
		foundAiTraces, err := legacyRows(ctx, query, args, scanLegacyAiTrace)
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
	var count uint64
	if err = chdb.Conn.QueryRow(ctx, "SELECT count(), toDateTime64(min(recorded_at), 3, 'UTC'), toDateTime64(max(recorded_at), 3, 'UTC') FROM "+table).Scan(&count, &oldest, &newest); err != nil {
		return oldest, newest, false, err
	}
	return oldest, newest, count > 0, nil
}

// FindMovedIds returns full occurrence keys, never IDs shared by unrelated traces.
func (legacyRepository) FindMovedIds(ctx context.Context, table string, ids []uuid.UUID, from, to time.Time) (map[shared.MovedOccurrence]bool, error) {
	table, err := shared.LegacyTable(table)
	if err != nil {
		return nil, err
	}
	moved := map[shared.MovedOccurrence]bool{}
	for chunk := range slices.Chunk(ids, legacyIdChunk) {
		found, err := legacyRows(ctx, "SELECT project_id, id, trace_id, span_id, recorded_at FROM "+table+" WHERE recorded_at >= ? AND recorded_at < ? AND id IN (?)", []any{from, to, chunk}, func(rows driver.Rows) (shared.MovedOccurrence, error) {
			var project, id uuid.UUID
			var trace, span string
			var at time.Time
			err := rows.Scan(&project, &id, &trace, &span, &at)
			return shared.MovedOccurrenceKey(project, id, trace, span, at), err
		})
		if err != nil {
			return nil, err
		}
		for _, id := range found {
			moved[id] = true
		}
	}
	return moved, nil
}

func (legacyRepository) FindMovedSpans(ctx context.Context, traceIds []string, from, to time.Time) (map[string]bool, error) {
	moved := map[string]bool{}
	for chunk := range slices.Chunk(traceIds, legacyIdChunk) {
		found, err := legacyRows(ctx, "SELECT project_id, trace_id, span_id FROM "+shared.SpansTable+" WHERE recorded_at >= ? AND recorded_at < ? AND trace_id IN (?)", []any{from, to, chunk}, func(rows driver.Rows) (string, error) {
			var project uuid.UUID
			var trace, span string
			err := rows.Scan(&project, &trace, &span)
			return shared.MovedSpanKey(project, trace, span), err
		})
		if err != nil {
			return nil, err
		}
		for _, key := range found {
			moved[key] = true
		}
	}
	return moved, nil
}

// OccurrenceTime matches the destination column precision for recovery keys.
func (legacyRepository) OccurrenceTime(table string, at time.Time) time.Time {
	if table == "ai_traces" {
		return at.Truncate(time.Millisecond)
	}
	return at.Truncate(time.Microsecond)
}

var LegacyRepository = legacyRepository{}
