//go:build !telemetry_ch && !telemetry_duckdb && !telemetry_firebolt

package sqlite

import (
	"context"
	"fmt"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type aiTraceRowNaming struct{ lit.DefaultDbNamingStrategy }

func (aiTraceRowNaming) GetTableNameFromStructName(string) string {
	return "ai_traces"
}

type aiTraceRow struct {
	Id                 uuid.UUID                 `lit:"id"`
	ProjectId          uuid.UUID                 `lit:"project_id"`
	RecordedAt         sqlitetypes.SQLiteTime    `lit:"recorded_at"`
	Duration           int64                     `lit:"duration"`
	StatusCode         uint8                     `lit:"status_code"`
	Model              string                    `lit:"model"`
	ResponseModel      string                    `lit:"response_model"`
	Provider           string                    `lit:"provider"`
	Operation          string                    `lit:"operation"`
	InputTokens        int64                     `lit:"input_tokens"`
	OutputTokens       int64                     `lit:"output_tokens"`
	TotalTokens        int64                     `lit:"total_tokens"`
	CachedTokens       int64                     `lit:"cached_tokens"`
	ReasoningTokens    int64                     `lit:"reasoning_tokens"`
	InputCost          float64                   `lit:"input_cost"`
	OutputCost         float64                   `lit:"output_cost"`
	TotalCost          float64                   `lit:"total_cost"`
	TraceName          string                    `lit:"trace_name"`
	UserId             string                    `lit:"user_id"`
	FinishReason       string                    `lit:"finish_reason"`
	ServerName         string                    `lit:"server_name"`
	AppVersion         string                    `lit:"app_version"`
	StorageKey         string                    `lit:"storage_key"`
	Attributes         sqlitetypes.SQLiteJSONMap `lit:"attributes"`
	DistributedTraceId *uuid.UUID                `lit:"distributed_trace_id"`
	IsRoot             bool                      `lit:"is_root"`
	ConversationId     string                    `lit:"conversation_id"`
	ToolCallCount      int64                     `lit:"tool_call_count"`
	ToolNames          string                    `lit:"tool_names"`
	Flagged            bool                      `lit:"flagged"`
	FlaggedTerms       string                    `lit:"flagged_terms"`
}

type groupedAiTraceRow struct {
	TraceName       string  `lit:"trace_name"`
	TotalCount      uint64  `lit:"total_count"`
	AvgDuration     float64 `lit:"avg_duration"`
	TotalTokens     int64   `lit:"total_tokens"`
	TotalCost       float64 `lit:"total_cost"`
	AvgInputTokens  float64 `lit:"avg_input_tokens"`
	AvgOutputTokens float64 `lit:"avg_output_tokens"`
	LastSeen        string  `lit:"last_seen"`
	HasRoot         bool    `lit:"has_root"`
	HasNonRoot      bool    `lit:"has_non_root"`
}

type aiTraceDurationRow struct {
	Duration float64 `lit:"duration"`
}

type aiTraceDetailStatsRow struct {
	Count           int64   `lit:"count"`
	AvgDurationMs   float64 `lit:"avg_duration_ms"`
	TotalTokens     int64   `lit:"total_tokens"`
	TotalCost       float64 `lit:"total_cost"`
	AvgInputTokens  float64 `lit:"avg_input_tokens"`
	AvgOutputTokens float64 `lit:"avg_output_tokens"`
}

type conversationStatsRow struct {
	ConversationId string  `lit:"conversation_id"`
	UserId         string  `lit:"user_id"`
	Turns          int64   `lit:"turns"`
	TotalTokens    int64   `lit:"total_tokens"`
	TotalCost      float64 `lit:"total_cost"`
	ToolCallCount  int64   `lit:"tool_call_count"`
	ToolNames      string  `lit:"tool_names"`
	Models         string  `lit:"models"`
	Flagged        bool    `lit:"flagged"`
	FlaggedTerms   string  `lit:"flagged_terms"`
	FirstSeen      string  `lit:"first_seen"`
	LastSeen       string  `lit:"last_seen"`
}

type userConversationRow struct {
	UserId      string  `lit:"user_id"`
	Turns       int64   `lit:"turns"`
	ConvCost    float64 `lit:"conv_cost"`
	ConvTokens  int64   `lit:"conv_tokens"`
	ConvFlagged bool    `lit:"conv_flagged"`
	LastSeen    string  `lit:"last_seen"`
}

type conversationCostRow struct {
	ConversationId string  `lit:"conversation_id"`
	TotalCost      float64 `lit:"total_cost"`
}

type modelNameRow struct {
	Model string `lit:"model"`
}

type toolNamesRow struct {
	ToolNames string `lit:"tool_names"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModelWithNaming[aiTraceRow](driver, aiTraceRowNaming{})
		lit.RegisterModel[groupedAiTraceRow](driver)
		lit.RegisterModel[aiTraceDurationRow](driver)
		lit.RegisterModel[aiTraceDetailStatsRow](driver)
		lit.RegisterModel[conversationStatsRow](driver)
		lit.RegisterModel[userConversationRow](driver)
		lit.RegisterModel[conversationCostRow](driver)
		lit.RegisterModel[modelNameRow](driver)
		lit.RegisterModel[toolNamesRow](driver)
	})
}

func aiTraceToRow(t models.AiTrace) aiTraceRow {
	return aiTraceRow{
		Id:                 t.Id,
		ProjectId:          t.ProjectId,
		RecordedAt:         sqlitetypes.NewSQLiteTime(t.RecordedAt),
		Duration:           int64(t.Duration),
		StatusCode:         t.StatusCode,
		Model:              t.Model,
		ResponseModel:      t.ResponseModel,
		Provider:           t.Provider,
		Operation:          t.Operation,
		InputTokens:        t.InputTokens,
		OutputTokens:       t.OutputTokens,
		TotalTokens:        t.TotalTokens,
		CachedTokens:       t.CachedTokens,
		ReasoningTokens:    t.ReasoningTokens,
		InputCost:          t.InputCost,
		OutputCost:         t.OutputCost,
		TotalCost:          t.TotalCost,
		TraceName:          t.TraceName,
		UserId:             t.UserId,
		FinishReason:       t.FinishReason,
		ServerName:         t.ServerName,
		AppVersion:         t.AppVersion,
		StorageKey:         t.StorageKey,
		Attributes:         sqlitetypes.NewSQLiteJSONMap(t.Attributes),
		DistributedTraceId: t.DistributedTraceId,
		IsRoot:             t.IsRoot,
		ConversationId:     t.ConversationId,
		ToolCallCount:      t.ToolCallCount,
		ToolNames:          shared.JoinCSV(t.ToolNames),
		Flagged:            t.Flagged,
		FlaggedTerms:       shared.JoinCSV(t.FlaggedTerms),
	}
}

func (r *aiTraceRow) toModel() models.AiTrace {
	t := models.AiTrace{
		Id:                 r.Id,
		ProjectId:          r.ProjectId,
		RecordedAt:         r.RecordedAt.Time,
		Duration:           time.Duration(r.Duration),
		StatusCode:         r.StatusCode,
		Model:              r.Model,
		ResponseModel:      r.ResponseModel,
		Provider:           r.Provider,
		Operation:          r.Operation,
		InputTokens:        r.InputTokens,
		OutputTokens:       r.OutputTokens,
		TotalTokens:        r.TotalTokens,
		CachedTokens:       r.CachedTokens,
		ReasoningTokens:    r.ReasoningTokens,
		InputCost:          r.InputCost,
		OutputCost:         r.OutputCost,
		TotalCost:          r.TotalCost,
		TraceName:          r.TraceName,
		UserId:             r.UserId,
		FinishReason:       r.FinishReason,
		ServerName:         r.ServerName,
		AppVersion:         r.AppVersion,
		StorageKey:         r.StorageKey,
		DistributedTraceId: r.DistributedTraceId,
		IsRoot:             r.IsRoot,
		ConversationId:     r.ConversationId,
		ToolCallCount:      r.ToolCallCount,
		ToolNames:          shared.SplitCSV(r.ToolNames),
		Flagged:            r.Flagged,
		FlaggedTerms:       shared.SplitCSV(r.FlaggedTerms),
	}
	if r.Attributes != nil {
		t.Attributes = map[string]string(r.Attributes)
	}
	return t
}

type aiTraceRepository struct{}

func (r *aiTraceRepository) InsertAsync(ctx context.Context, lines []models.AiTrace) error {
	if len(lines) == 0 {
		return nil
	}

	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, t := range lines {
		row := aiTraceToRow(t)
		if err := lit.InsertExistingUuid(tx, &row); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *aiTraceRepository) FindGroupedByTraceName(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy, sortDirection, search, rootFilter string) ([]models.AiTraceStats, int64, error) {
	params := lit.P{"project_id": projectId, "from": sqlitetypes.NewSQLiteTime(fromDate), "to": sqlitetypes.NewSQLiteTime(toDate)}

	whereClause := "project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to"
	if search != "" {
		whereClause += " AND INSTR(LOWER(trace_name), LOWER(:search)) > 0"
		params["search"] = search
	}
	whereClause += shared.RootFilterClause("is_root", rootFilter)

	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(DISTINCT trace_name) AS count FROM ai_traces WHERE "+whereClause, params)
	if err != nil {
		return nil, 0, err
	}
	count := int64(0)
	if countResult != nil {
		count = int64(countResult.Count)
	}

	rows, err := lit.SelectNamed[groupedAiTraceRow](db.TelemetryDB,
		`SELECT trace_name, COUNT(*) AS total_count,
			AVG(duration) AS avg_duration,
			SUM(total_tokens) AS total_tokens,
			SUM(total_cost) AS total_cost,
			AVG(input_tokens) AS avg_input_tokens,
			AVG(output_tokens) AS avg_output_tokens,
			MAX(recorded_at) AS last_seen,
			MAX(is_root) AS has_root,
			MAX(CASE WHEN is_root = 0 THEN 1 ELSE 0 END) AS has_non_root
		FROM ai_traces WHERE `+whereClause+`
		GROUP BY trace_name`, params)
	if err != nil {
		return nil, 0, err
	}

	var stats []models.AiTraceStats
	for _, row := range rows {
		// Compute percentiles from raw durations for this trace_name
		durationParams := lit.P{"project_id": projectId, "from": sqlitetypes.NewSQLiteTime(fromDate), "to": sqlitetypes.NewSQLiteTime(toDate), "trace_name": row.TraceName}
		durationRows, err := lit.SelectNamed[aiTraceDurationRow](db.TelemetryDB,
			`SELECT CAST(duration AS REAL) AS duration FROM ai_traces
			WHERE project_id = :project_id AND trace_name = :trace_name AND recorded_at >= :from AND recorded_at <= :to
			ORDER BY duration ASC`, durationParams)
		if err != nil {
			return nil, 0, err
		}

		sortedDurations := make([]float64, len(durationRows))
		for i, d := range durationRows {
			sortedDurations[i] = d.Duration
		}

		lastSeen, _ := time.Parse(time.RFC3339Nano, row.LastSeen)

		stats = append(stats, models.AiTraceStats{
			TraceName:       row.TraceName,
			Count:           row.TotalCount,
			P50Duration:     time.Duration(shared.ComputePercentile(sortedDurations, 0.5)),
			P95Duration:     time.Duration(shared.ComputePercentile(sortedDurations, 0.95)),
			AvgDuration:     time.Duration(row.AvgDuration),
			TotalTokens:     row.TotalTokens,
			TotalCost:       row.TotalCost,
			AvgInputTokens:  row.AvgInputTokens,
			AvgOutputTokens: row.AvgOutputTokens,
			LastSeen:        lastSeen,
			HasRoot:         row.HasRoot,
			HasNonRoot:      row.HasNonRoot,
		})
	}

	// Sort results
	orderByMap := map[string]func(i, j int) bool{
		"count":        func(i, j int) bool { return stats[i].Count > stats[j].Count },
		"p50_duration": func(i, j int) bool { return stats[i].P50Duration > stats[j].P50Duration },
		"p95_duration": func(i, j int) bool { return stats[i].P95Duration > stats[j].P95Duration },
		"avg_duration": func(i, j int) bool { return stats[i].AvgDuration > stats[j].AvgDuration },
		"total_tokens": func(i, j int) bool { return stats[i].TotalTokens > stats[j].TotalTokens },
		"total_cost":   func(i, j int) bool { return stats[i].TotalCost > stats[j].TotalCost },
		"last_seen":    func(i, j int) bool { return stats[i].LastSeen.After(stats[j].LastSeen) },
	}

	sortFn, ok := orderByMap[orderBy]
	if !ok {
		sortFn = orderByMap["total_cost"]
	}

	if sortDirection == "asc" {
		origFn := sortFn
		sortFn = func(i, j int) bool { return !origFn(i, j) }
	}
	sort.Slice(stats, sortFn)

	// Paginate
	offset := (page - 1) * pageSize
	end := offset + pageSize
	if offset > len(stats) {
		stats = nil
	} else if end > len(stats) {
		stats = stats[offset:]
	} else {
		stats = stats[offset:end]
	}

	return stats, count, nil
}

func (r *aiTraceRepository) FindByTraceName(ctx context.Context, projectId uuid.UUID, traceName string, fromDate, toDate time.Time, page, pageSize int, orderBy, sortDirection string) ([]models.AiTrace, int64, error) {
	params := lit.P{"project_id": projectId, "trace_name": traceName, "from": sqlitetypes.NewSQLiteTime(fromDate), "to": sqlitetypes.NewSQLiteTime(toDate)}

	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM ai_traces WHERE project_id = :project_id AND trace_name = :trace_name AND recorded_at >= :from AND recorded_at <= :to",
		params)
	if err != nil {
		return nil, 0, err
	}
	count := int64(0)
	if countResult != nil {
		count = int64(countResult.Count)
	}

	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{
		"recorded_at": true, "duration": true, "total_tokens": true,
		"total_cost": true, "input_tokens": true, "output_tokens": true,
	}
	if !allowedOrderBy[orderBy] {
		orderBy = "recorded_at"
	}
	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	rows, err := lit.SelectNamed[aiTraceRow](db.TelemetryDB,
		fmt.Sprintf(`SELECT id, project_id, recorded_at, duration, status_code,
			model, response_model, provider, operation,
			input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens,
			input_cost, output_cost, total_cost,
			trace_name, user_id, finish_reason, server_name, app_version,
			storage_key, attributes, distributed_trace_id, is_root,
			conversation_id, tool_call_count, tool_names, flagged, flagged_terms
		FROM ai_traces
		WHERE project_id = :project_id AND trace_name = :trace_name AND recorded_at >= :from AND recorded_at <= :to
		ORDER BY %s %s LIMIT :limit OFFSET :offset`, orderBy, sortDir),
		lit.P{"project_id": projectId, "trace_name": traceName, "from": sqlitetypes.NewSQLiteTime(fromDate), "to": sqlitetypes.NewSQLiteTime(toDate), "limit": pageSize, "offset": offset})
	if err != nil {
		return nil, 0, err
	}

	traces := make([]models.AiTrace, 0, len(rows))
	for _, row := range rows {
		traces = append(traces, row.toModel())
	}

	return traces, count, nil
}

func (r *aiTraceRepository) GetTraceNameStats(ctx context.Context, projectId uuid.UUID, traceName string, start, end time.Time) (*models.AiTraceDetailStats, error) {
	durationMinutes := end.Sub(start).Minutes()
	if durationMinutes < 1 {
		durationMinutes = 1
	}

	params := lit.P{"project_id": projectId, "trace_name": traceName, "from": sqlitetypes.NewSQLiteTime(start), "to": sqlitetypes.NewSQLiteTime(end)}

	row, err := lit.SelectSingleNamed[aiTraceDetailStatsRow](db.TelemetryDB,
		`SELECT COUNT(*) AS count,
			CASE WHEN COUNT(*) > 0 THEN AVG(duration) / 1000000.0 ELSE 0 END AS avg_duration_ms,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
			COALESCE(SUM(total_cost), 0) AS total_cost,
			COALESCE(AVG(input_tokens), 0) AS avg_input_tokens,
			COALESCE(AVG(output_tokens), 0) AS avg_output_tokens
		FROM ai_traces
		WHERE project_id = :project_id AND trace_name = :trace_name AND recorded_at >= :from AND recorded_at <= :to`,
		params)
	if err != nil {
		return nil, err
	}

	stats := &models.AiTraceDetailStats{}
	if row != nil {
		stats.Count = row.Count
		stats.AvgDuration = row.AvgDurationMs
		stats.TotalTokens = row.TotalTokens
		stats.TotalCost = row.TotalCost
		stats.AvgInputTokens = row.AvgInputTokens
		stats.AvgOutputTokens = row.AvgOutputTokens
		stats.Throughput = float64(row.Count) / durationMinutes
	}

	// Compute median and p95 from raw durations
	durationRows, err := lit.SelectNamed[aiTraceDurationRow](db.TelemetryDB,
		`SELECT CAST(duration AS REAL) / 1000000.0 AS duration FROM ai_traces
		WHERE project_id = :project_id AND trace_name = :trace_name AND recorded_at >= :from AND recorded_at <= :to
		ORDER BY duration ASC`, params)
	if err != nil {
		return stats, nil
	}

	sortedDurations := make([]float64, len(durationRows))
	for i, d := range durationRows {
		sortedDurations[i] = d.Duration
	}
	stats.MedianDuration = shared.ComputePercentile(sortedDurations, 0.5)
	stats.P95Duration = shared.ComputePercentile(sortedDurations, 0.95)

	return stats, nil
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
		WHERE project_id = :project_id AND id = :id`
	params := lit.P{"project_id": projectId, "id": traceId}
	if recordedAt != nil {
		from, to := shared.TraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = sqlitetypes.NewSQLiteTime(from)
		params["to"] = sqlitetypes.NewSQLiteTime(to)
	}
	query += ` LIMIT 1`

	row, err := lit.SelectSingleNamed[aiTraceRow](db.TelemetryDB, query, params)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	result := row.toModel()
	return &result, nil
}

func (r *aiTraceRepository) FindByDistributedTraceId(ctx context.Context, distributedTraceId uuid.UUID, projectIds []uuid.UUID, recordedAt *time.Time) ([]models.AiTrace, error) {
	if len(projectIds) == 0 {
		return nil, nil
	}
	params := lit.P{"trace_id": distributedTraceId}
	placeholders := make([]string, len(projectIds))
	for i, pid := range projectIds {
		key := fmt.Sprintf("pid_%d", i)
		placeholders[i] = ":" + key
		params[key] = pid
	}
	query := `SELECT id, project_id, recorded_at, duration, status_code,
			model, response_model, provider, operation,
			input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens,
			input_cost, output_cost, total_cost,
			trace_name, user_id, finish_reason, server_name, app_version,
			storage_key, attributes, distributed_trace_id, is_root,
			conversation_id, tool_call_count, tool_names, flagged, flagged_terms
		FROM ai_traces WHERE distributed_trace_id = :trace_id AND project_id IN (` + strings.Join(placeholders, ",") + `)`
	if recordedAt != nil {
		from, to := shared.DistributedTraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = sqlitetypes.NewSQLiteTime(from)
		params["to"] = sqlitetypes.NewSQLiteTime(to)
	}
	query += ` ORDER BY recorded_at ASC`

	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return nil, err
	}
	sqlRows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	var traces []models.AiTrace
	for sqlRows.Next() {
		var row aiTraceRow
		if err := sqlRows.Scan(
			&row.Id, &row.ProjectId, &row.RecordedAt, &row.Duration, &row.StatusCode,
			&row.Model, &row.ResponseModel, &row.Provider, &row.Operation,
			&row.InputTokens, &row.OutputTokens, &row.TotalTokens, &row.CachedTokens, &row.ReasoningTokens,
			&row.InputCost, &row.OutputCost, &row.TotalCost,
			&row.TraceName, &row.UserId, &row.FinishReason, &row.ServerName, &row.AppVersion,
			&row.StorageKey, &row.Attributes, &row.DistributedTraceId, &row.IsRoot,
			&row.ConversationId, &row.ToolCallCount, &row.ToolNames, &row.Flagged, &row.FlaggedTerms,
		); err != nil {
			return nil, err
		}
		traces = append(traces, row.toModel())
	}
	return traces, nil
}

func (r *aiTraceRepository) FindConversations(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy, sortDirection, search, userId, model, toolName string, flaggedOnly bool) ([]models.AiConversationStats, int64, *models.AiConversationThresholds, error) {
	params := lit.P{"project_id": projectId, "from": sqlitetypes.NewSQLiteTime(fromDate), "to": sqlitetypes.NewSQLiteTime(toDate)}

	baseWhere := "project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to AND conversation_id != ''"

	// Row-level filters use a semi-join on conversation_id: a conversation
	// matches when ANY of its turns matches, and its aggregates still cover
	// all turns (a plain WHERE would drop the non-matching turns from the
	// sums).
	var rowPredicates []string
	if search != "" {
		rowPredicates = append(rowPredicates,
			"(INSTR(LOWER(conversation_id), LOWER(:search)) > 0 OR INSTR(LOWER(user_id), LOWER(:search)) > 0 OR INSTR(LOWER(model), LOWER(:search)) > 0 OR INSTR(LOWER(tool_names), LOWER(:search)) > 0 OR INSTR(LOWER(flagged_terms), LOWER(:search)) > 0)")
		params["search"] = search
	}
	if userId != "" {
		rowPredicates = append(rowPredicates, "user_id = :user_id")
		params["user_id"] = userId
	}
	if model != "" {
		rowPredicates = append(rowPredicates, "model = :model")
		params["model"] = model
	}
	if toolName != "" {
		rowPredicates = append(rowPredicates, "INSTR(',' || tool_names || ',', ',' || :tool_name || ',') > 0")
		params["tool_name"] = toolName
	}

	whereClause := baseWhere
	if len(rowPredicates) > 0 {
		whereClause += " AND conversation_id IN (SELECT DISTINCT conversation_id FROM ai_traces WHERE " +
			baseWhere + " AND " + strings.Join(rowPredicates, " AND ") + ")"
	}

	havingClause := ""
	if flaggedOnly {
		havingClause = " HAVING MAX(flagged) = 1"
	}

	rows, err := lit.SelectNamed[conversationStatsRow](db.TelemetryDB,
		`SELECT conversation_id,
			MAX(user_id) AS user_id,
			COUNT(*) AS turns,
			SUM(total_tokens) AS total_tokens,
			SUM(total_cost) AS total_cost,
			SUM(tool_call_count) AS tool_call_count,
			GROUP_CONCAT(DISTINCT tool_names) AS tool_names,
			GROUP_CONCAT(DISTINCT model) AS models,
			MAX(flagged) AS flagged,
			GROUP_CONCAT(DISTINCT flagged_terms) AS flagged_terms,
			MIN(recorded_at) AS first_seen,
			MAX(recorded_at) AS last_seen
		FROM ai_traces WHERE `+whereClause+`
		GROUP BY conversation_id`+havingClause, params)
	if err != nil {
		return nil, 0, nil, err
	}

	stats := make([]models.AiConversationStats, 0, len(rows))
	for _, row := range rows {
		firstSeen, _ := time.Parse(time.RFC3339Nano, row.FirstSeen)
		lastSeen, _ := time.Parse(time.RFC3339Nano, row.LastSeen)
		stats = append(stats, models.AiConversationStats{
			ConversationId: row.ConversationId,
			UserId:         row.UserId,
			Turns:          row.Turns,
			TotalTokens:    row.TotalTokens,
			TotalCost:      row.TotalCost,
			ToolCallCount:  row.ToolCallCount,
			ToolNames:      shared.UnionCSV(row.ToolNames),
			Models:         shared.UnionCSV(row.Models),
			Flagged:        row.Flagged,
			FlaggedTerms:   shared.UnionCSV(row.FlaggedTerms),
			FirstSeen:      firstSeen,
			LastSeen:       lastSeen,
		})
	}

	total := int64(len(stats))
	thresholds := shared.ConversationThresholds(stats)
	stats = shared.SortAndPageConversations(stats, orderBy, sortDirection, page, pageSize)
	return stats, total, thresholds, nil
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
		WHERE project_id = :project_id AND conversation_id = :conversation_id`
	params := lit.P{"project_id": projectId, "conversation_id": conversationId}
	if !fromDate.IsZero() {
		query += ` AND recorded_at >= :from`
		params["from"] = sqlitetypes.NewSQLiteTime(fromDate)
	}
	if !toDate.IsZero() {
		query += ` AND recorded_at <= :to`
		params["to"] = sqlitetypes.NewSQLiteTime(toDate)
	}
	query += ` ORDER BY recorded_at ASC LIMIT 1000`

	rows, err := lit.SelectNamed[aiTraceRow](db.TelemetryDB, query, params)
	if err != nil {
		return nil, nil, err
	}

	traces := make([]models.AiTrace, 0, len(rows))
	for _, row := range rows {
		traces = append(traces, row.toModel())
	}
	return traces, shared.BuildConversationDetailStats(traces), nil
}

func (r *aiTraceRepository) FindUserStats(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy, sortDirection, search string) ([]models.AiUserStats, int64, error) {
	params := lit.P{"project_id": projectId, "from": sqlitetypes.NewSQLiteTime(fromDate), "to": sqlitetypes.NewSQLiteTime(toDate)}

	whereClause := "project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to AND user_id != '' AND conversation_id != ''"
	if search != "" {
		whereClause += " AND INSTR(LOWER(user_id), LOWER(:search)) > 0"
		params["search"] = search
	}

	rows, err := lit.SelectNamed[userConversationRow](db.TelemetryDB,
		`SELECT user_id,
			COUNT(*) AS turns,
			SUM(total_cost) AS conv_cost,
			SUM(total_tokens) AS conv_tokens,
			MAX(flagged) AS conv_flagged,
			MAX(recorded_at) AS last_seen
		FROM ai_traces WHERE `+whereClause+`
		GROUP BY user_id, conversation_id`, params)
	if err != nil {
		return nil, 0, err
	}

	aggRows := make([]shared.UserConversationAgg, 0, len(rows))
	for _, row := range rows {
		lastSeen, _ := time.Parse(time.RFC3339Nano, row.LastSeen)
		aggRows = append(aggRows, shared.UserConversationAgg{
			UserId:   row.UserId,
			Turns:    row.Turns,
			Cost:     row.ConvCost,
			Tokens:   row.ConvTokens,
			Flagged:  row.ConvFlagged,
			LastSeen: lastSeen,
		})
	}

	stats := shared.AggregateUserStats(aggRows)
	total := int64(len(stats))
	stats = shared.SortAndPageUserStats(stats, orderBy, sortDirection, page, pageSize)
	return stats, total, nil
}

func (r *aiTraceRepository) GetConversationCosts(ctx context.Context, projectId uuid.UUID, conversationIds []string, since time.Time) (map[string]float64, error) {
	costs := make(map[string]float64, len(conversationIds))
	if len(conversationIds) == 0 {
		return costs, nil
	}
	params := lit.P{"project_id": projectId, "since": sqlitetypes.NewSQLiteTime(since)}
	placeholders := make([]string, len(conversationIds))
	for i, id := range conversationIds {
		key := fmt.Sprintf("cid_%d", i)
		placeholders[i] = ":" + key
		params[key] = id
	}
	rows, err := lit.SelectNamed[conversationCostRow](db.TelemetryDB,
		`SELECT conversation_id, SUM(total_cost) AS total_cost
		FROM ai_traces
		WHERE project_id = :project_id AND recorded_at >= :since AND conversation_id IN (`+strings.Join(placeholders, ",")+`)
		GROUP BY conversation_id`, params)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		costs[row.ConversationId] = row.TotalCost
	}
	return costs, nil
}

func (r *aiTraceRepository) ListModels(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time) ([]string, error) {
	rows, err := lit.SelectNamed[modelNameRow](db.TelemetryDB,
		`SELECT DISTINCT model FROM ai_traces
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to AND model != ''
		ORDER BY model LIMIT 200`,
		lit.P{"project_id": projectId, "from": sqlitetypes.NewSQLiteTime(fromDate), "to": sqlitetypes.NewSQLiteTime(toDate)})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Model)
	}
	return names, nil
}

func (r *aiTraceRepository) ListToolNames(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time) ([]string, error) {
	rows, err := lit.SelectNamed[toolNamesRow](db.TelemetryDB,
		`SELECT DISTINCT tool_names FROM ai_traces
		WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to AND tool_names != ''
		LIMIT 500`,
		lit.P{"project_id": projectId, "from": sqlitetypes.NewSQLiteTime(fromDate), "to": sqlitetypes.NewSQLiteTime(toDate)})
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.ToolNames)
	}
	return shared.UnionCSV(shared.JoinCSV(values)), nil
}

var AiTraceRepository = &aiTraceRepository{}
