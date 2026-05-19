//go:build !pgch

package repositories

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type aiTraceRow struct {
	Id                 uuid.UUID      `lit:"id"`
	ProjectId          uuid.UUID      `lit:"project_id"`
	DistributedTraceId *uuid.UUID     `lit:"distributed_trace_id"`
	RecordedAt         SQLiteTime     `lit:"recorded_at"`
	Duration           int64          `lit:"duration"`
	StatusCode         uint8          `lit:"status_code"`
	Model              string         `lit:"model"`
	ResponseModel      string         `lit:"response_model"`
	Provider           string         `lit:"provider"`
	Operation          string         `lit:"operation"`
	InputTokens        int64          `lit:"input_tokens"`
	OutputTokens       int64          `lit:"output_tokens"`
	TotalTokens        int64          `lit:"total_tokens"`
	CachedTokens       int64          `lit:"cached_tokens"`
	ReasoningTokens    int64          `lit:"reasoning_tokens"`
	InputCost          float64        `lit:"input_cost"`
	OutputCost         float64        `lit:"output_cost"`
	TotalCost          float64        `lit:"total_cost"`
	TraceName          string         `lit:"trace_name"`
	UserId             string         `lit:"user_id"`
	FinishReason       string         `lit:"finish_reason"`
	ServerName         string         `lit:"server_name"`
	AppVersion         string         `lit:"app_version"`
	StorageKey         string         `lit:"storage_key"`
	Attributes         SQLiteJSONMap  `lit:"attributes"`
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

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[aiTraceRow](driver)
		lit.RegisterModel[groupedAiTraceRow](driver)
		lit.RegisterModel[aiTraceDurationRow](driver)
		lit.RegisterModel[aiTraceDetailStatsRow](driver)
	})
}

func aiTraceToRow(t models.AiTrace) aiTraceRow {
	return aiTraceRow{
		Id:                 t.Id,
		ProjectId:          t.ProjectId,
		DistributedTraceId: t.DistributedTraceId,
		RecordedAt:         NewSQLiteTime(t.RecordedAt),
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
		Attributes:         NewSQLiteJSONMap(t.Attributes),
	}
}

func (r *aiTraceRow) toModel() models.AiTrace {
	t := models.AiTrace{
		Id:                 r.Id,
		ProjectId:          r.ProjectId,
		DistributedTraceId: r.DistributedTraceId,
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
	for _, t := range lines {
		row := aiTraceToRow(t)
		if err := lit.InsertExistingUuid(db.TelemetryDB, &row); err != nil {
			return err
		}
	}
	return nil
}

func (r *aiTraceRepository) FindGroupedByTraceName(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy, sortDirection, search string) ([]models.AiTraceStats, int64, error) {
	params := lit.P{"project_id": projectId, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate)}

	whereClause := "project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to"
	if search != "" {
		whereClause += " AND INSTR(LOWER(trace_name), LOWER(:search)) > 0"
		params["search"] = search
	}

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
			MAX(recorded_at) AS last_seen
		FROM ai_traces WHERE `+whereClause+`
		GROUP BY trace_name`, params)
	if err != nil {
		return nil, 0, err
	}

	var stats []models.AiTraceStats
	for _, row := range rows {
		// Compute percentiles from raw durations for this trace_name
		durationParams := lit.P{"project_id": projectId, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate), "trace_name": row.TraceName}
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

		lastSeen, _ := time.Parse("2006-01-02 15:04:05", row.LastSeen)

		stats = append(stats, models.AiTraceStats{
			TraceName:       row.TraceName,
			Count:           row.TotalCount,
			P50Duration:     time.Duration(computePercentile(sortedDurations, 0.5)),
			P95Duration:     time.Duration(computePercentile(sortedDurations, 0.95)),
			AvgDuration:     time.Duration(row.AvgDuration),
			TotalTokens:     row.TotalTokens,
			TotalCost:       row.TotalCost,
			AvgInputTokens:  row.AvgInputTokens,
			AvgOutputTokens: row.AvgOutputTokens,
			LastSeen:        lastSeen,
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
	params := lit.P{"project_id": projectId, "trace_name": traceName, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate)}

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
			storage_key, attributes
		FROM ai_traces
		WHERE project_id = :project_id AND trace_name = :trace_name AND recorded_at >= :from AND recorded_at <= :to
		ORDER BY %s %s LIMIT :limit OFFSET :offset`, orderBy, sortDir),
		lit.P{"project_id": projectId, "trace_name": traceName, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate), "limit": pageSize, "offset": offset})
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

	params := lit.P{"project_id": projectId, "trace_name": traceName, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)}

	row, err := lit.SelectSingleNamed[aiTraceDetailStatsRow](db.TelemetryDB,
		`SELECT COUNT(*) AS count,
			CASE WHEN COUNT(*) > 0 THEN AVG(duration) / 1000000.0 ELSE 0 END AS avg_duration_ms,
			SUM(total_tokens) AS total_tokens,
			SUM(total_cost) AS total_cost,
			AVG(input_tokens) AS avg_input_tokens,
			AVG(output_tokens) AS avg_output_tokens
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
	stats.MedianDuration = computePercentile(sortedDurations, 0.5)
	stats.P95Duration = computePercentile(sortedDurations, 0.95)

	return stats, nil
}

func (r *aiTraceRepository) FindById(ctx context.Context, projectId, traceId uuid.UUID) (*models.AiTrace, error) {
	row, err := lit.SelectSingleNamed[aiTraceRow](db.TelemetryDB,
		`SELECT id, project_id, recorded_at, duration, status_code,
			model, response_model, provider, operation,
			input_tokens, output_tokens, total_tokens, cached_tokens, reasoning_tokens,
			input_cost, output_cost, total_cost,
			trace_name, user_id, finish_reason, server_name, app_version,
			storage_key, attributes
		FROM ai_traces
		WHERE project_id = :project_id AND id = :id
		LIMIT 1`,
		lit.P{"project_id": projectId, "id": traceId})
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	result := row.toModel()
	return &result, nil
}

var AiTraceRepository = aiTraceRepository{}
