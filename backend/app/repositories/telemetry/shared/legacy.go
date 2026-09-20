package shared

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type legacyReadExecutor struct {
	*sql.DB
	ctx context.Context
}

func (r legacyReadExecutor) Query(query string, args ...any) (*sql.Rows, error) {
	return r.DB.QueryContext(r.ctx, query, args...)
}

// lit.SelectNamed uses Query without a context; the adapter keeps shutdown from
// waiting for an uncancellable legacy scan.
func SelectLegacy[T any](ctx context.Context, conn *sql.DB, query string, params lit.P) ([]*T, error) {
	return lit.SelectNamed[T](legacyReadExecutor{conn, ctx}, query, params)
}

// The move-over reads the tables V2 replaced. Endpoints, tasks, AI traces and exceptions come back in their V2 model
// with the old ids parked in the new fields exactly as they were stored: on an entity TraceId holds the old distributed
// trace id and SpanId the old span id, on an exception TraceId holds the old owner id and LinkedTraceId the old
// distributed trace id. The moveover package turns them into V2 ids, in one place for every backend.

// LegacySpan is a row of the old spans table, where trace_id named the owning endpoint, task or AI trace.
type LegacySpan struct {
	ProjectId    uuid.UUID
	Id           uuid.UUID
	OwnerId      uuid.UUID
	ParentSpanId string
	Name         string
	StartTime    time.Time
	Duration     time.Duration
	RecordedAt   time.Time
	Attributes   map[string]string
}

// LegacyTables maps each table V2 replaced to the table its rows move into.
var LegacyTables = map[string]string{
	"endpoints":              "endpoints_v2",
	"tasks":                  "tasks_v2",
	"ai_traces":              "ai_traces_v2",
	"exception_stack_traces": "exceptions_v2",
	"spans":                  SpansTable,
}

// LegacyTable guards the table names the move-over puts into SQL text.
func LegacyTable(name string) (string, error) {
	if _, known := LegacyTables[name]; known {
		return name, nil
	}
	for _, v2 := range LegacyTables {
		if v2 == name {
			return name, nil
		}
	}
	return "", fmt.Errorf("%q is not a table the move-over reads or writes", name)
}

// MovedSpanKey identifies a span already present in the V2 table when a day is resumed.
func MovedSpanKey(projectId uuid.UUID, traceId, spanId string) string {
	return OtelKey(projectId, traceId) + ":" + spanId
}

// Legacy history is immutable after cutover. Offset paging within a single second
// preserves even identical retries; ordering every copied value makes unequal rows
// stable across reads. An ID alone cannot order occurrences from different traces.
func LegacyPageOrder(table string, clickhouse bool) string {
	columns := map[string]string{
		"endpoints":              "id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name, distributed_trace_id, span_id, is_stream, is_root",
		"tasks":                  "id, project_id, task_name, duration, recorded_at, client_ip, attributes, app_version, server_name, distributed_trace_id, span_id, is_root",
		"ai_traces":              "id, project_id, recorded_at, duration, status_code, model, response_model, provider, operation, input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens, input_cost, output_cost, total_cost, trace_name, user_id, finish_reason, server_name, app_version, storage_key, attributes, distributed_trace_id, is_root, conversation_id, tool_call_count, tool_names, flagged, flagged_terms",
		"exception_stack_traces": "id, project_id, trace_id, trace_type, exception_hash, stack_trace, recorded_at, attributes, app_version, server_name, is_message, distributed_trace_id, session_id",
		"spans":                  "project_id, id, trace_id, parent_span_id, name, start_time, duration, recorded_at, attributes",
	}[table]
	if clickhouse {
		// Arrays and nullable UUIDs need a common, totally ordered representation.
		return "toJSONString(tuple(" + columns + "))"
	}
	return columns
}

type MovedOccurrence struct {
	ProjectId       uuid.UUID
	Id              uuid.UUID
	TraceId, SpanId string
	RecordedAt      string
}

func MovedOccurrenceKey(project, id uuid.UUID, trace, span string, at time.Time) MovedOccurrence {
	return MovedOccurrence{project, id, trace, span, at.UTC().Format(time.RFC3339Nano)}
}
