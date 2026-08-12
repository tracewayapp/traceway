//go:build telemetry_ch

package clickhouse

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type aiTraceRepository struct{}

func (r *aiTraceRepository) InsertAsync(ctx context.Context, lines []models.AiTrace) error {
	batch, err := chdb.Conn.PrepareBatch(chdb.BatchCtx(),
		"INSERT INTO ai_traces (id, project_id, recorded_at, duration, status_code, model, response_model, provider, operation, input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens, input_cost, output_cost, total_cost, trace_name, user_id, finish_reason, server_name, app_version, storage_key, attributes, distributed_trace_id, is_root, conversation_id, tool_call_count, tool_names, flagged, flagged_terms)")
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
		flagged := uint8(0)
		if t.Flagged {
			flagged = 1
		}
		if err := batch.Append(
			t.Id, t.ProjectId, t.RecordedAt, int64(t.Duration), t.StatusCode,
			t.Model, t.ResponseModel, t.Provider, t.Operation,
			t.InputTokens, t.OutputTokens, t.TotalTokens, t.CachedTokens, t.ReasoningTokens,
			t.InputCost, t.OutputCost, t.TotalCost,
			t.TraceName, t.UserId, t.FinishReason, t.ServerName, t.AppVersion,
			t.StorageKey, attributesJSON, t.DistributedTraceId, isRoot,
			t.ConversationId, t.ToolCallCount, shared.JoinCSV(t.ToolNames), flagged, shared.JoinCSV(t.FlaggedTerms),
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
		storage_key, attributes, distributed_trace_id, is_root,
		conversation_id, tool_call_count, tool_names, flagged, flagged_terms
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
		t, err := scanAiTrace(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		traces = append(traces, t)
	}

	return traces, int64(count), nil
}

// scanAiTrace scans the canonical full ai_traces column list (see
// aiTraceColumns) into a model, decoding the JSON/CSV/flag columns.
func scanAiTrace(scan func(dest ...any) error) (models.AiTrace, error) {
	var t models.AiTrace
	var attributesJSON, toolNames, flaggedTerms string
	var isRoot, flagged uint8
	if err := scan(
		&t.Id, &t.ProjectId, &t.RecordedAt, &t.Duration, &t.StatusCode,
		&t.Model, &t.ResponseModel, &t.Provider, &t.Operation,
		&t.InputTokens, &t.OutputTokens, &t.TotalTokens, &t.CachedTokens, &t.ReasoningTokens,
		&t.InputCost, &t.OutputCost, &t.TotalCost,
		&t.TraceName, &t.UserId, &t.FinishReason, &t.ServerName, &t.AppVersion,
		&t.StorageKey, &attributesJSON, &t.DistributedTraceId, &isRoot,
		&t.ConversationId, &t.ToolCallCount, &toolNames, &flagged, &flaggedTerms,
	); err != nil {
		return t, err
	}
	t.IsRoot = isRoot == 1
	t.Flagged = flagged == 1
	t.ToolNames = shared.SplitCSV(toolNames)
	t.FlaggedTerms = shared.SplitCSV(flaggedTerms)
	if attributesJSON != "" && attributesJSON != "{}" {
		if err := json.Unmarshal([]byte(attributesJSON), &t.Attributes); err != nil {
			t.Attributes = nil
		}
	}
	return t, nil
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
		storage_key, attributes, distributed_trace_id, is_root,
		conversation_id, tool_call_count, tool_names, flagged, flagged_terms
	FROM ai_traces
	WHERE project_id = ? AND id = ?`
	args := []any{projectId, traceId}
	if recordedAt != nil {
		from, to := shared.TraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= ? AND recorded_at <= ?`
		args = append(args, from, to)
	}
	query += ` LIMIT 1`

	row := chdb.Conn.QueryRow(ctx, query, args...)
	t, err := scanAiTrace(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
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
		storage_key, attributes, distributed_trace_id, is_root,
		conversation_id, tool_call_count, tool_names, flagged, flagged_terms
	FROM ai_traces
	WHERE distributed_trace_id = ? AND project_id IN (` + strings.Join(placeholders, ",") + `)`
	if recordedAt != nil {
		from, to := shared.DistributedTraceWindowBounds(*recordedAt)
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
		t, err := scanAiTrace(rows.Scan)
		if err != nil {
			return nil, err
		}
		traces = append(traces, t)
	}
	return traces, nil
}

func (r *aiTraceRepository) FindConversations(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy, sortDirection, search, userId, model, toolName string, flaggedOnly bool) ([]models.AiConversationStats, int64, *models.AiConversationThresholds, error) {
	// WHERE columns are table-qualified: the select list aliases aggregates to
	// the same names (max(user_id) AS user_id), and ClickHouse resolves bare
	// names in WHERE against those aliases.
	whereClause := "ai_traces.project_id = ? AND ai_traces.recorded_at >= ? AND ai_traces.recorded_at <= ? AND ai_traces.conversation_id != ''"
	args := []interface{}{projectId, fromDate, toDate}

	// Row-level filters use a semi-join on conversation_id: a conversation
	// matches when ANY of its turns matches, and its aggregates still cover
	// all turns (a plain WHERE would drop the non-matching turns from the
	// sums). The subquery has its own scope, so bare column names are safe
	// there.
	var rowPredicates []string
	var rowArgs []interface{}
	if search != "" {
		rowPredicates = append(rowPredicates,
			"(positionCaseInsensitive(conversation_id, ?) > 0 OR positionCaseInsensitive(user_id, ?) > 0 OR positionCaseInsensitive(model, ?) > 0 OR positionCaseInsensitive(tool_names, ?) > 0 OR positionCaseInsensitive(flagged_terms, ?) > 0)")
		rowArgs = append(rowArgs, search, search, search, search, search)
	}
	if userId != "" {
		rowPredicates = append(rowPredicates, "user_id = ?")
		rowArgs = append(rowArgs, userId)
	}
	if model != "" {
		rowPredicates = append(rowPredicates, "model = ?")
		rowArgs = append(rowArgs, model)
	}
	if toolName != "" {
		rowPredicates = append(rowPredicates, "position(concat(',', tool_names, ','), concat(',', ?, ',')) > 0")
		rowArgs = append(rowArgs, toolName)
	}
	if len(rowPredicates) > 0 {
		whereClause += " AND ai_traces.conversation_id IN (SELECT DISTINCT conversation_id FROM ai_traces WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? AND conversation_id != '' AND " +
			strings.Join(rowPredicates, " AND ") + ")"
		args = append(args, projectId, fromDate, toDate)
		args = append(args, rowArgs...)
	}

	havingClause := ""
	if flaggedOnly {
		// References the max(flagged) select alias: ClickHouse rejects
		// repeating the aggregate inside HAVING as nested aggregation.
		havingClause = " HAVING flagged = 1"
	}

	groupedQuery := `SELECT
			conversation_id,
			max(user_id) AS user_id,
			count() AS turns,
			sum(total_tokens) AS total_tokens,
			sum(total_cost) AS total_cost,
			sum(tool_call_count) AS tool_call_count,
			arrayStringConcat(groupUniqArray(tool_names), ',') AS tool_names,
			arrayStringConcat(groupUniqArray(model), ',') AS models,
			max(flagged) AS flagged,
			arrayStringConcat(groupUniqArray(flagged_terms), ',') AS flagged_terms,
			min(recorded_at) AS first_seen,
			max(recorded_at) AS last_seen
		FROM ai_traces
		WHERE ` + whereClause + `
		GROUP BY conversation_id` + havingClause

	var count uint64
	if err := chdb.Conn.QueryRow(ctx,
		"SELECT count() FROM ("+groupedQuery+")", args...).Scan(&count); err != nil {
		return nil, 0, nil, err
	}

	thresholds := &models.AiConversationThresholds{}
	if err := chdb.Conn.QueryRow(ctx,
		"SELECT quantile(0.95)(total_cost), quantile(0.95)(turns) FROM ("+groupedQuery+")",
		args...).Scan(&thresholds.P95Cost, &thresholds.P95Turns); err != nil {
		return nil, 0, nil, err
	}

	orderByMap := map[string]string{
		"turns":           "turns",
		"total_cost":      "total_cost",
		"total_tokens":    "total_tokens",
		"tool_call_count": "tool_call_count",
		"user_id":         "user_id",
		"first_seen":      "first_seen",
		"last_seen":       "last_seen",
	}
	orderExpr, ok := orderByMap[orderBy]
	if !ok {
		orderExpr = "last_seen"
	}
	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}
	offset := (page - 1) * pageSize

	rows, err := chdb.Conn.Query(ctx,
		groupedQuery+` ORDER BY `+orderExpr+` `+sortDir+` LIMIT ? OFFSET ?`,
		append(append([]interface{}{}, args...), pageSize, offset)...)
	if err != nil {
		return nil, 0, nil, err
	}
	defer rows.Close()

	var stats []models.AiConversationStats
	for rows.Next() {
		var s models.AiConversationStats
		var turns uint64
		var toolNames, modelsCSV, flaggedTerms string
		var flagged uint8
		if err := rows.Scan(
			&s.ConversationId, &s.UserId, &turns,
			&s.TotalTokens, &s.TotalCost, &s.ToolCallCount,
			&toolNames, &modelsCSV, &flagged, &flaggedTerms,
			&s.FirstSeen, &s.LastSeen,
		); err != nil {
			return nil, 0, nil, err
		}
		s.Turns = int64(turns)
		s.ToolNames = shared.UnionCSV(toolNames)
		s.Models = shared.UnionCSV(modelsCSV)
		s.Flagged = flagged == 1
		s.FlaggedTerms = shared.UnionCSV(flaggedTerms)
		stats = append(stats, s)
	}

	return stats, int64(count), thresholds, nil
}

func (r *aiTraceRepository) FindByConversationId(ctx context.Context, projectId uuid.UUID, conversationId string, fromDate, toDate time.Time) ([]models.AiTrace, *models.AiConversationDetailStats, error) {
	query := `SELECT id, project_id, recorded_at, duration, status_code,
		model, response_model, provider, operation,
		input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens,
		input_cost, output_cost, total_cost,
		trace_name, user_id, finish_reason, server_name, app_version,
		storage_key, attributes, distributed_trace_id, is_root,
		conversation_id, tool_call_count, tool_names, flagged, flagged_terms
	FROM ai_traces
	WHERE project_id = ? AND conversation_id = ?`
	args := []interface{}{projectId, conversationId}
	if !fromDate.IsZero() {
		query += ` AND recorded_at >= ?`
		args = append(args, fromDate)
	}
	if !toDate.IsZero() {
		query += ` AND recorded_at <= ?`
		args = append(args, toDate)
	}
	query += ` ORDER BY recorded_at ASC LIMIT 1000`

	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var traces []models.AiTrace
	for rows.Next() {
		t, err := scanAiTrace(rows.Scan)
		if err != nil {
			return nil, nil, err
		}
		traces = append(traces, t)
	}
	return traces, shared.BuildConversationDetailStats(traces), nil
}

func (r *aiTraceRepository) FindUserStats(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy, sortDirection, search string) ([]models.AiUserStats, int64, error) {
	whereClause := "project_id = ? AND recorded_at >= ? AND recorded_at <= ? AND user_id != '' AND conversation_id != ''"
	args := []interface{}{projectId, fromDate, toDate}
	if search != "" {
		whereClause += " AND positionCaseInsensitive(user_id, ?) > 0"
		args = append(args, search)
	}

	var count uint64
	if err := chdb.Conn.QueryRow(ctx,
		"SELECT uniq(user_id) FROM ai_traces WHERE "+whereClause, args...).Scan(&count); err != nil {
		return nil, 0, err
	}

	orderByMap := map[string]string{
		"conversation_count":         "conversation_count",
		"total_calls":                "total_calls",
		"avg_turns":                  "avg_turns",
		"min_turns":                  "min_turns",
		"median_turns":               "median_turns",
		"avg_conversation_cost":      "avg_conversation_cost",
		"total_cost":                 "total_cost",
		"flagged_conversation_count": "flagged_conversation_count",
		"total_tokens":               "total_tokens",
		"last_seen":                  "last_seen",
	}
	orderExpr, ok := orderByMap[orderBy]
	if !ok {
		orderExpr = "total_cost"
	}
	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}
	offset := (page - 1) * pageSize

	query := `SELECT
			user_id,
			count() AS conversation_count,
			sum(turns) AS total_calls,
			avg(turns) AS avg_turns,
			min(turns) AS min_turns,
			quantile(0.5)(turns) AS median_turns,
			avg(conv_cost) AS avg_conversation_cost,
			sum(conv_cost) AS total_cost,
			sum(conv_flagged) AS flagged_conversation_count,
			sum(conv_tokens) AS total_tokens,
			max(conv_last_seen) AS last_seen
		FROM (
			SELECT user_id, conversation_id,
				count() AS turns,
				sum(total_cost) AS conv_cost,
				sum(total_tokens) AS conv_tokens,
				max(flagged) AS conv_flagged,
				max(recorded_at) AS conv_last_seen
			FROM ai_traces
			WHERE ` + whereClause + `
			GROUP BY user_id, conversation_id
		)
		GROUP BY user_id
		ORDER BY ` + orderExpr + ` ` + sortDir + `
		LIMIT ? OFFSET ?`

	rows, err := chdb.Conn.Query(ctx, query, append(append([]interface{}{}, args...), pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var stats []models.AiUserStats
	for rows.Next() {
		var s models.AiUserStats
		var conversationCount, totalCalls, minTurns, flaggedCount uint64
		if err := rows.Scan(
			&s.UserId, &conversationCount, &totalCalls,
			&s.AvgTurns, &minTurns, &s.MedianTurns,
			&s.AvgCostPerConversation, &s.TotalCost,
			&flaggedCount, &s.TotalTokens, &s.LastSeen,
		); err != nil {
			return nil, 0, err
		}
		s.ConversationCount = int64(conversationCount)
		s.TotalCalls = int64(totalCalls)
		s.MinTurns = int64(minTurns)
		s.FlaggedConversationCount = int64(flaggedCount)
		stats = append(stats, s)
	}

	return stats, int64(count), nil
}

func (r *aiTraceRepository) GetConversationCosts(ctx context.Context, projectId uuid.UUID, conversationIds []string, since time.Time) (map[string]float64, error) {
	costs := make(map[string]float64, len(conversationIds))
	if len(conversationIds) == 0 {
		return costs, nil
	}
	placeholders := make([]string, len(conversationIds))
	args := []interface{}{projectId, since}
	for i, id := range conversationIds {
		placeholders[i] = "?"
		args = append(args, id)
	}

	rows, err := chdb.Conn.Query(ctx,
		`SELECT conversation_id, sum(total_cost)
		FROM ai_traces
		WHERE project_id = ? AND recorded_at >= ? AND conversation_id IN (`+strings.Join(placeholders, ",")+`)
		GROUP BY conversation_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var conversationId string
		var totalCost float64
		if err := rows.Scan(&conversationId, &totalCost); err != nil {
			return nil, err
		}
		costs[conversationId] = totalCost
	}
	return costs, nil
}

func (r *aiTraceRepository) ListModels(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time) ([]string, error) {
	rows, err := chdb.Conn.Query(ctx,
		`SELECT DISTINCT model FROM ai_traces
		WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? AND model != ''
		ORDER BY model LIMIT 200`, projectId, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

func (r *aiTraceRepository) ListToolNames(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time) ([]string, error) {
	rows, err := chdb.Conn.Query(ctx,
		`SELECT DISTINCT tool_names FROM ai_traces
		WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? AND tool_names != ''
		LIMIT 500`, projectId, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return shared.UnionCSV(shared.JoinCSV(values)), nil
}

var AiTraceRepository = &aiTraceRepository{}
