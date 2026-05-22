//go:build pgch

package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type aiTraceRepository struct{}

func (r *aiTraceRepository) InsertAsync(ctx context.Context, lines []models.AiTrace) error {
	batch, err := chdb.Conn.PrepareBatch(clickhouse.Context(context.Background(), clickhouse.WithAsync(false)),
		"INSERT INTO ai_traces (id, project_id, recorded_at, duration, status_code, model, response_model, provider, operation, input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens, input_cost, output_cost, total_cost, trace_name, user_id, finish_reason, server_name, app_version, storage_key, attributes, trace_id, span_id, parent_span_id, distributed_trace_id)")
	if err != nil {
		return err
	}
	for _, t := range lines {
		attributesJSON := "{}"
		if len(t.Attributes) != 0 {
			if attributesBytes, err := json.Marshal(t.Attributes); err == nil {
				attributesJSON = string(attributesBytes)
			}
		}
		if err := batch.Append(
			t.Id, t.ProjectId, t.RecordedAt, int64(t.Duration), t.StatusCode,
			t.Model, t.ResponseModel, t.Provider, t.Operation,
			t.InputTokens, t.OutputTokens, t.TotalTokens, t.CachedTokens, t.ReasoningTokens,
			t.InputCost, t.OutputCost, t.TotalCost,
			t.TraceName, t.UserId, t.FinishReason, t.ServerName, t.AppVersion,
			t.StorageKey, attributesJSON,
			t.TraceId, t.SpanId, t.ParentSpanId, t.DistributedTraceId,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (r *aiTraceRepository) FindGroupedByTraceName(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy, sortDirection, search string, isRoot *bool) ([]models.AiTraceStats, int64, error) {
	whereClause := "project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{projectId, fromDate, toDate}

	if search != "" {
		whereClause += " AND positionCaseInsensitive(trace_name, ?) > 0"
		args = append(args, search)
	}

	if isRoot != nil {
		if *isRoot {
			whereClause += " AND parent_span_id IS NULL"
		} else {
			whereClause += " AND parent_span_id IS NOT NULL"
		}
	}

	var count uint64
	err := chdb.Conn.QueryRow(ctx, "SELECT uniq(trace_name) FROM ai_traces WHERE "+whereClause, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize

	orderByMap := map[string]string{
		"count":        "count",
		"p50_duration": "p50_duration",
		"p95_duration": "p95_duration",
		"avg_duration": "avg_duration",
		"total_tokens": "total_tokens",
		"total_cost":   "total_cost",
		"last_seen":    "last_seen",
	}
	orderExpr, ok := orderByMap[orderBy]
	if !ok {
		orderExpr = "total_cost"
	}

	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	query := `SELECT
		trace_name,
		count() as count,
		countIf(parent_span_id IS NOT NULL) as non_root_count,
		quantile(0.5)(duration) as p50_duration,
		quantile(0.95)(duration) as p95_duration,
		avg(duration) as avg_duration,
		sum(total_tokens) as total_tokens,
		sum(total_cost) as total_cost,
		avg(input_tokens) as avg_input_tokens,
		avg(output_tokens) as avg_output_tokens,
		max(recorded_at) as last_seen
	FROM ai_traces
	WHERE ` + whereClause + `
	GROUP BY trace_name
	ORDER BY ` + orderExpr + ` ` + sortDir + `
	LIMIT ? OFFSET ?`

	queryArgs := append(args, pageSize, offset)
	rows, err := chdb.Conn.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var stats []models.AiTraceStats
	for rows.Next() {
		var s models.AiTraceStats
		var p50, p95, avg float64
		if err := rows.Scan(&s.TraceName, &s.Count, &s.NonRootCount, &p50, &p95, &avg,
			&s.TotalTokens, &s.TotalCost, &s.AvgInputTokens, &s.AvgOutputTokens, &s.LastSeen); err != nil {
			return nil, 0, err
		}
		s.P50Duration = time.Duration(p50)
		s.P95Duration = time.Duration(p95)
		s.AvgDuration = time.Duration(avg)
		stats = append(stats, s)
	}

	return stats, int64(count), nil
}

func (r *aiTraceRepository) FindByTraceName(ctx context.Context, projectId uuid.UUID, traceName string, fromDate, toDate time.Time, page, pageSize int, orderBy, sortDirection string) ([]models.AiTrace, int64, error) {
	var count uint64
	err := chdb.Conn.QueryRow(ctx, "SELECT count() FROM ai_traces WHERE project_id = ? AND trace_name = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, traceName, fromDate, toDate).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{
		"recorded_at":   true,
		"duration":      true,
		"total_tokens":  true,
		"total_cost":    true,
		"input_tokens":  true,
		"output_tokens": true,
	}
	if !allowedOrderBy[orderBy] {
		orderBy = "recorded_at"
	}

	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	query := `SELECT id, project_id, recorded_at, duration, status_code,
		model, response_model, provider, operation,
		input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens,
		input_cost, output_cost, total_cost,
		trace_name, user_id, finish_reason, server_name, app_version,
		storage_key, attributes,
		trace_id, span_id, parent_span_id, distributed_trace_id
	FROM ai_traces
	WHERE project_id = ? AND trace_name = ? AND recorded_at >= ? AND recorded_at <= ?
	ORDER BY ` + orderBy + ` ` + sortDir + `
	LIMIT ? OFFSET ?`

	rows, err := chdb.Conn.Query(ctx, query, projectId, traceName, fromDate, toDate, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var traces []models.AiTrace
	for rows.Next() {
		var t models.AiTrace
		var attributesJSON string
		if err := rows.Scan(
			&t.Id, &t.ProjectId, &t.RecordedAt, &t.Duration, &t.StatusCode,
			&t.Model, &t.ResponseModel, &t.Provider, &t.Operation,
			&t.InputTokens, &t.OutputTokens, &t.TotalTokens, &t.CachedTokens, &t.ReasoningTokens,
			&t.InputCost, &t.OutputCost, &t.TotalCost,
			&t.TraceName, &t.UserId, &t.FinishReason, &t.ServerName, &t.AppVersion,
			&t.StorageKey, &attributesJSON,
			&t.TraceId, &t.SpanId, &t.ParentSpanId, &t.DistributedTraceId,
		); err != nil {
			return nil, 0, err
		}
		if attributesJSON != "" && attributesJSON != "{}" {
			if err := json.Unmarshal([]byte(attributesJSON), &t.Attributes); err != nil {
				t.Attributes = nil
			}
		}
		traces = append(traces, t)
	}

	return traces, int64(count), nil
}

func (r *aiTraceRepository) GetTraceNameStats(ctx context.Context, projectId uuid.UUID, traceName string, start, end time.Time) (*models.AiTraceDetailStats, error) {
	durationMinutes := end.Sub(start).Minutes()
	if durationMinutes < 1 {
		durationMinutes = 1
	}

	query := `SELECT
		count() as count,
		if(count() > 0, avg(duration) / 1000000, 0) as avg_duration_ms,
		if(count() > 0, quantile(0.5)(duration) / 1000000, 0) as median_duration_ms,
		if(count() > 0, quantile(0.95)(duration) / 1000000, 0) as p95_duration_ms,
		sum(total_tokens) as total_tokens,
		sum(total_cost) as total_cost,
		avg(input_tokens) as avg_input_tokens,
		avg(output_tokens) as avg_output_tokens
	FROM ai_traces
	WHERE project_id = ? AND trace_name = ? AND recorded_at >= ? AND recorded_at <= ?`

	var stats models.AiTraceDetailStats
	var count uint64

	err := chdb.Conn.QueryRow(ctx, query, projectId, traceName, start, end).Scan(
		&count,
		&stats.AvgDuration,
		&stats.MedianDuration,
		&stats.P95Duration,
		&stats.TotalTokens,
		&stats.TotalCost,
		&stats.AvgInputTokens,
		&stats.AvgOutputTokens,
	)
	if err != nil {
		return nil, err
	}

	stats.Count = int64(count)
	stats.Throughput = float64(count) / durationMinutes

	return &stats, nil
}

// FindByParentSpanIds returns ai_trace rows whose parent_span_id is in the
// given set, projected into ChildEntityRef. Used by FindChildEntitiesBySpanIds
// to surface gen_ai children nested under a parent entity's subtree — closes
// the AI-link-icon regression that Phase 2's entity_id filter introduced.
func (r *aiTraceRepository) FindByParentSpanIds(ctx context.Context, projectId uuid.UUID, spanIds []uuid.UUID) ([]ChildEntityRef, error) {
	if len(spanIds) == 0 {
		return nil, nil
	}

	query := `SELECT id, trace_name, parent_span_id, trace_id, recorded_at, duration
		FROM ai_traces
		WHERE project_id = ? AND parent_span_id IN (?)`

	rows, err := chdb.Conn.Query(ctx, query, projectId, spanIds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []ChildEntityRef
	for rows.Next() {
		var (
			id           uuid.UUID
			name         string
			parentSpanId *uuid.UUID
			traceId      uuid.UUID
			recordedAt   time.Time
			durationNs   int64
		)
		if err := rows.Scan(&id, &name, &parentSpanId, &traceId, &recordedAt, &durationNs); err != nil {
			return nil, err
		}
		if parentSpanId == nil {
			continue
		}
		refs = append(refs, ChildEntityRef{
			Kind: "ai_trace", Id: id, Name: name,
			ParentSpanId: *parentSpanId, TraceId: traceId,
			RecordedAt: recordedAt, Duration: time.Duration(durationNs),
		})
	}
	return refs, nil
}

func (r *aiTraceRepository) FindById(ctx context.Context, projectId, traceId uuid.UUID) (*models.AiTrace, error) {
	query := `SELECT id, project_id, recorded_at, duration, status_code,
		model, response_model, provider, operation,
		input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens,
		input_cost, output_cost, total_cost,
		trace_name, user_id, finish_reason, server_name, app_version,
		storage_key, attributes,
		trace_id, span_id, parent_span_id, distributed_trace_id
	FROM ai_traces
	WHERE project_id = ? AND id = ?
	LIMIT 1`

	var t models.AiTrace
	var attributesJSON string

	err := chdb.Conn.QueryRow(ctx, query, projectId, traceId).Scan(
		&t.Id, &t.ProjectId, &t.RecordedAt, &t.Duration, &t.StatusCode,
		&t.Model, &t.ResponseModel, &t.Provider, &t.Operation,
		&t.InputTokens, &t.OutputTokens, &t.TotalTokens, &t.CachedTokens, &t.ReasoningTokens,
		&t.InputCost, &t.OutputCost, &t.TotalCost,
		&t.TraceName, &t.UserId, &t.FinishReason, &t.ServerName, &t.AppVersion,
		&t.StorageKey, &attributesJSON,
		&t.TraceId, &t.SpanId, &t.ParentSpanId, &t.DistributedTraceId,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if attributesJSON != "" && attributesJSON != "{}" {
		if err := json.Unmarshal([]byte(attributesJSON), &t.Attributes); err != nil {
			t.Attributes = nil
		}
	}

	return &t, nil
}

func (r *aiTraceRepository) FindByDistributedTraceId(ctx context.Context, distributedTraceId uuid.UUID, projectIds []uuid.UUID) ([]models.AiTrace, error) {
	query := `SELECT id, project_id, recorded_at, duration, status_code,
		model, response_model, provider, operation,
		input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens,
		input_cost, output_cost, total_cost,
		trace_name, user_id, finish_reason, server_name, app_version,
		storage_key, attributes,
		trace_id, span_id, parent_span_id, distributed_trace_id
	FROM ai_traces
	WHERE distributed_trace_id = ? AND project_id IN (?)
	ORDER BY recorded_at ASC`

	rows, err := chdb.Conn.Query(ctx, query, distributedTraceId, projectIds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traces []models.AiTrace
	for rows.Next() {
		var t models.AiTrace
		var attributesJSON string
		if err := rows.Scan(
			&t.Id, &t.ProjectId, &t.RecordedAt, &t.Duration, &t.StatusCode,
			&t.Model, &t.ResponseModel, &t.Provider, &t.Operation,
			&t.InputTokens, &t.OutputTokens, &t.TotalTokens, &t.CachedTokens, &t.ReasoningTokens,
			&t.InputCost, &t.OutputCost, &t.TotalCost,
			&t.TraceName, &t.UserId, &t.FinishReason, &t.ServerName, &t.AppVersion,
			&t.StorageKey, &attributesJSON,
			&t.TraceId, &t.SpanId, &t.ParentSpanId, &t.DistributedTraceId,
		); err != nil {
			return nil, err
		}
		if attributesJSON != "" && attributesJSON != "{}" {
			if err := json.Unmarshal([]byte(attributesJSON), &t.Attributes); err != nil {
				t.Attributes = nil
			}
		}
		traces = append(traces, t)
	}
	return traces, nil
}

func (r *aiTraceRepository) FindBySpanIds(ctx context.Context, projectId uuid.UUID, spanIds []uuid.UUID) (map[uuid.UUID]AiTraceRef, error) {
	result := map[uuid.UUID]AiTraceRef{}
	if len(spanIds) == 0 {
		return result, nil
	}

	query := `SELECT span_id, id, trace_name
		FROM ai_traces
		WHERE project_id = ? AND span_id IN (?)`

	rows, err := chdb.Conn.Query(ctx, query, projectId, spanIds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var spanId *uuid.UUID
		var ref AiTraceRef
		if err := rows.Scan(&spanId, &ref.Id, &ref.TraceName); err != nil {
			return nil, err
		}
		if spanId != nil {
			result[*spanId] = ref
		}
	}
	return result, nil
}

var AiTraceRepository = aiTraceRepository{}
