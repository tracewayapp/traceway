//go:build pgch

package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type aiTraceRepository struct{}

func (r *aiTraceRepository) InsertAsync(ctx context.Context, lines []models.AiTrace) error {
	batch, err := chdb.Conn.PrepareBatch(chdb.BatchCtx(),
		"INSERT INTO ai_traces (id, project_id, recorded_at, duration, status_code, model, response_model, provider, operation, input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens, input_cost, output_cost, total_cost, trace_name, user_id, finish_reason, server_name, app_version, storage_key, attributes, distributed_trace_id, is_root)")
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
		isRoot := uint8(0)
		if t.IsRoot {
			isRoot = 1
		}
		if err := batch.Append(
			t.Id, t.ProjectId, t.RecordedAt, int64(t.Duration), t.StatusCode,
			t.Model, t.ResponseModel, t.Provider, t.Operation,
			t.InputTokens, t.OutputTokens, t.TotalTokens, t.CachedTokens, t.ReasoningTokens,
			t.InputCost, t.OutputCost, t.TotalCost,
			t.TraceName, t.UserId, t.FinishReason, t.ServerName, t.AppVersion,
			t.StorageKey, attributesJSON, t.DistributedTraceId, isRoot,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (r *aiTraceRepository) FindGroupedByTraceName(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy, sortDirection, search, rootFilter string) ([]models.AiTraceStats, int64, error) {
	whereClause := "project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	args := []interface{}{projectId, fromDate, toDate}

	if search != "" {
		whereClause += " AND positionCaseInsensitive(trace_name, ?) > 0"
		args = append(args, search)
	}
	switch rootFilter {
	case "root":
		whereClause += " AND is_root = 1"
	case "non_root":
		whereClause += " AND is_root = 0"
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
		quantile(0.5)(duration) as p50_duration,
		quantile(0.95)(duration) as p95_duration,
		avg(duration) as avg_duration,
		sum(total_tokens) as total_tokens,
		sum(total_cost) as total_cost,
		avg(input_tokens) as avg_input_tokens,
		avg(output_tokens) as avg_output_tokens,
		max(recorded_at) as last_seen,
		max(is_root) as has_root,
		max(if(is_root = 0, 1, 0)) as has_non_root
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
		var hasRoot, hasNonRoot uint8
		if err := rows.Scan(&s.TraceName, &s.Count, &p50, &p95, &avg,
			&s.TotalTokens, &s.TotalCost, &s.AvgInputTokens, &s.AvgOutputTokens, &s.LastSeen, &hasRoot, &hasNonRoot); err != nil {
			return nil, 0, err
		}
		s.P50Duration = time.Duration(p50)
		s.P95Duration = time.Duration(p95)
		s.AvgDuration = time.Duration(avg)
		s.HasRoot = hasRoot == 1
		s.HasNonRoot = hasNonRoot == 1
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
		storage_key, attributes
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

func (r *aiTraceRepository) FindById(ctx context.Context, projectId, traceId uuid.UUID, recordedAt *time.Time) (*models.AiTrace, error) {
	query := `SELECT id, project_id, recorded_at, duration, status_code,
		model, response_model, provider, operation,
		input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens,
		input_cost, output_cost, total_cost,
		trace_name, user_id, finish_reason, server_name, app_version,
		storage_key, attributes, distributed_trace_id, is_root
	FROM ai_traces
	WHERE project_id = ? AND id = ?`
	args := []any{projectId, traceId}
	if recordedAt != nil {
		from, to := traceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= ? AND recorded_at <= ?`
		args = append(args, from, to)
	}
	query += ` LIMIT 1`

	var t models.AiTrace
	var attributesJSON string
	var isRoot uint8

	err := chdb.Conn.QueryRow(ctx, query, args...).Scan(
		&t.Id, &t.ProjectId, &t.RecordedAt, &t.Duration, &t.StatusCode,
		&t.Model, &t.ResponseModel, &t.Provider, &t.Operation,
		&t.InputTokens, &t.OutputTokens, &t.TotalTokens, &t.CachedTokens, &t.ReasoningTokens,
		&t.InputCost, &t.OutputCost, &t.TotalCost,
		&t.TraceName, &t.UserId, &t.FinishReason, &t.ServerName, &t.AppVersion,
		&t.StorageKey, &attributesJSON, &t.DistributedTraceId, &isRoot,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	t.IsRoot = isRoot == 1

	if attributesJSON != "" && attributesJSON != "{}" {
		if err := json.Unmarshal([]byte(attributesJSON), &t.Attributes); err != nil {
			t.Attributes = nil
		}
	}

	return &t, nil
}

func (r *aiTraceRepository) FindByDistributedTraceId(ctx context.Context, distributedTraceId uuid.UUID, projectIds []uuid.UUID, recordedAt *time.Time) ([]models.AiTrace, error) {
	if len(projectIds) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(projectIds))
	args := []interface{}{distributedTraceId}
	for i, pid := range projectIds {
		placeholders[i] = "?"
		args = append(args, pid)
		_ = i
	}
	query := `SELECT id, project_id, recorded_at, duration, status_code,
		model, response_model, provider, operation,
		input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens,
		input_cost, output_cost, total_cost,
		trace_name, user_id, finish_reason, server_name, app_version,
		storage_key, attributes, distributed_trace_id, is_root
	FROM ai_traces
	WHERE distributed_trace_id = ? AND project_id IN (` + strings.Join(placeholders, ",") + `)`
	if recordedAt != nil {
		from, to := distributedTraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= ? AND recorded_at <= ?`
		args = append(args, from, to)
	}
	query += ` ORDER BY recorded_at ASC`

	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traces []models.AiTrace
	for rows.Next() {
		var t models.AiTrace
		var attributesJSON string
		var isRoot uint8
		if err := rows.Scan(
			&t.Id, &t.ProjectId, &t.RecordedAt, &t.Duration, &t.StatusCode,
			&t.Model, &t.ResponseModel, &t.Provider, &t.Operation,
			&t.InputTokens, &t.OutputTokens, &t.TotalTokens, &t.CachedTokens, &t.ReasoningTokens,
			&t.InputCost, &t.OutputCost, &t.TotalCost,
			&t.TraceName, &t.UserId, &t.FinishReason, &t.ServerName, &t.AppVersion,
			&t.StorageKey, &attributesJSON, &t.DistributedTraceId, &isRoot,
		); err != nil {
			return nil, err
		}
		t.IsRoot = isRoot == 1
		if attributesJSON != "" && attributesJSON != "{}" {
			if err := json.Unmarshal([]byte(attributesJSON), &t.Attributes); err != nil {
				t.Attributes = nil
			}
		}
		traces = append(traces, t)
	}
	return traces, nil
}

var AiTraceRepository = aiTraceRepository{}
