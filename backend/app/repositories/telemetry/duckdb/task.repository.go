//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
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

type task struct {
	Id                 uuid.UUID                 `lit:"id"`
	ProjectId          uuid.UUID                 `lit:"project_id"`
	TaskName           string                    `lit:"task_name"`
	Duration           int64                     `lit:"duration"`
	RecordedAt         sqlitetypes.SQLiteTime    `lit:"recorded_at"`
	ClientIP           string                    `lit:"client_ip"`
	Attributes         sqlitetypes.SQLiteJSONMap `lit:"attributes"`
	AppVersion         string                    `lit:"app_version"`
	ServerName         string                    `lit:"server_name"`
	DistributedTraceId *uuid.UUID                `lit:"distributed_trace_id"`
	SpanId             *uuid.UUID                `lit:"span_id"`
	IsRoot             bool                      `lit:"is_root"`
}

type taskGroupRow struct {
	TaskName    string    `lit:"task_name"`
	Count       uint64    `lit:"count"`
	AvgDuration float64   `lit:"avg_duration"`
	LastSeen    time.Time `lit:"last_seen"`
	HasRoot     bool      `lit:"has_root"`
	HasNonRoot  bool      `lit:"has_non_root"`
	P50         float64   `lit:"p50"`
	P95         float64   `lit:"p95"`
}

type taskCountStatsRow struct {
	Count    int64   `lit:"count"`
	AvgDurMs float64 `lit:"avg_dur_ms"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[task](driver)
		lit.RegisterModel[taskGroupRow](driver)
		lit.RegisterModel[taskCountStatsRow](driver)
	})
}

func (r *task) toModel() models.Task {
	t := models.Task{
		Id:                 r.Id,
		ProjectId:          r.ProjectId,
		TaskName:           r.TaskName,
		Duration:           time.Duration(r.Duration),
		RecordedAt:         r.RecordedAt.Time,
		ClientIP:           r.ClientIP,
		AppVersion:         r.AppVersion,
		ServerName:         r.ServerName,
		DistributedTraceId: r.DistributedTraceId,
		SpanId:             r.SpanId,
		IsRoot:             r.IsRoot,
	}
	if r.Attributes != nil {
		t.Attributes = map[string]string(r.Attributes)
	}
	return t
}

type taskRepository struct{}

func convertTasks(lines []models.Task) [][]driver.Value {
	rows := make([][]driver.Value, 0, len(lines))
	for _, t := range lines {
		attributesJSON, err := attrJSON(t.Attributes)
		if err != nil {
			captureDroppedRow("tasks", err)
			continue
		}

		rows = append(rows, []driver.Value{
			duckUUID(t.Id),
			duckUUID(t.ProjectId),
			t.TaskName,
			int64(t.Duration),
			t.RecordedAt.UTC(),
			t.ClientIP,
			attributesJSON,
			t.AppVersion,
			t.ServerName,
			nullableUUID(t.DistributedTraceId),
			nullableUUID(t.SpanId),
			boolToInt(t.IsRoot),
		})
	}
	return rows
}

func (e *taskRepository) InsertAsync(ctx context.Context, lines []models.Task) error {
	return insertRows(ctx, "tasks", convertTasks(lines))
}

func (e *taskRepository) CountBetween(ctx context.Context, projectId uuid.UUID, start, end time.Time) (int64, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to",
		lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()})
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return int64(result.Count), nil
}

func (e *taskRepository) FindAll(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy string) ([]models.Task, int64, error) {
	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to",
		lit.P{"project_id": projectId, "from": fromDate.UTC(), "to": toDate.UTC()})
	if err != nil {
		return nil, 0, err
	}
	count := int64(0)
	if countResult != nil {
		count = int64(countResult.Count)
	}

	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{"recorded_at": true, "duration": true}
	if !allowedOrderBy[orderBy] {
		orderBy = "recorded_at"
	}

	rows, err := lit.SelectNamed[task](db.TelemetryDB,
		fmt.Sprintf(`SELECT id, project_id, task_name, duration, recorded_at, client_ip, attributes, app_version, server_name, distributed_trace_id
		FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		ORDER BY %s DESC LIMIT :limit OFFSET :offset`, orderBy),
		lit.P{"project_id": projectId, "from": fromDate.UTC(), "to": toDate.UTC(), "limit": pageSize, "offset": offset})
	if err != nil {
		return nil, 0, err
	}

	tasks := make([]models.Task, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, row.toModel())
	}

	return tasks, count, nil
}

func (e *taskRepository) FindGroupedByTaskName(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy string, sortDirection string, search string, rootFilter string) ([]models.TaskStats, int64, error) {
	params := lit.P{"project_id": projectId, "from": fromDate.UTC(), "to": toDate.UTC()}
	whereClause := "project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to"
	if search != "" {
		whereClause += " AND INSTR(LOWER(task_name), LOWER(:search)) > 0"
		params["search"] = search
	}
	whereClause += shared.RootFilterClause("is_root", rootFilter)

	needsGoSort := orderBy == "p50_duration" || orderBy == "p95_duration" || orderBy == "impact"

	// The Go-sort path fetches every group, so the total falls out of the
	// result set; the COUNT(DISTINCT) pre-scan is only needed when the
	// database paginates.
	totalCount := int64(0)
	if !needsGoSort {
		totalResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
			"SELECT COUNT(DISTINCT task_name) AS count FROM tasks WHERE "+whereClause,
			params)
		if err != nil {
			return nil, 0, err
		}
		if totalResult != nil {
			totalCount = int64(totalResult.Count)
		}
	}

	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	offset := (page - 1) * pageSize

	var baseQuery string

	groupedCols := `task_name, COUNT(*) as count, AVG(duration) as avg_duration, MAX(recorded_at) as last_seen,
			MAX(is_root) as has_root, MAX(CASE WHEN is_root = 0 THEN 1 ELSE 0 END) as has_non_root,
			quantile_cont(duration, 0.5) as p50, quantile_cont(duration, 0.95) as p95`

	if needsGoSort {
		baseQuery = `SELECT ` + groupedCols + `
			FROM tasks WHERE ` + whereClause + `
			GROUP BY task_name`
	} else {
		orderExpr := map[string]string{"count": "count", "last_seen": "last_seen"}
		expr, ok := orderExpr[orderBy]
		if !ok {
			expr = "count"
		}
		baseQuery = fmt.Sprintf(`SELECT `+groupedCols+`
			FROM tasks WHERE `+whereClause+`
			GROUP BY task_name ORDER BY %s %s LIMIT :limit OFFSET :offset`, expr, sortDir)
		params["limit"] = pageSize
		params["offset"] = offset
	}

	groups, err := lit.SelectNamed[taskGroupRow](db.TelemetryDB, baseQuery, params)
	if err != nil {
		return nil, 0, err
	}

	var stats []models.TaskStats
	for _, g := range groups {
		stats = append(stats, models.TaskStats{
			TaskName:    g.TaskName,
			Count:       g.Count,
			P50Duration: time.Duration(g.P50),
			P95Duration: time.Duration(g.P95),
			AvgDuration: time.Duration(g.AvgDuration),
			LastSeen:    g.LastSeen,
			HasRoot:     g.HasRoot,
			HasNonRoot:  g.HasNonRoot,
		})
	}

	if needsGoSort {
		totalCount = int64(len(stats))
		switch orderBy {
		case "p50_duration":
			sort.Slice(stats, func(i, j int) bool {
				if sortDir == "ASC" {
					return stats[i].P50Duration < stats[j].P50Duration
				}
				return stats[i].P50Duration > stats[j].P50Duration
			})
		case "p95_duration":
			sort.Slice(stats, func(i, j int) bool {
				if sortDir == "ASC" {
					return stats[i].P95Duration < stats[j].P95Duration
				}
				return stats[i].P95Duration > stats[j].P95Duration
			})
		case "impact":
			sort.Slice(stats, func(i, j int) bool {
				impactI := float64(stats[i].Count) * float64(stats[i].P95Duration-stats[i].P50Duration)
				impactJ := float64(stats[j].Count) * float64(stats[j].P95Duration-stats[j].P50Duration)
				if sortDir == "ASC" {
					return impactI < impactJ
				}
				return impactI > impactJ
			})
		}

		end := offset + pageSize
		if end > len(stats) {
			end = len(stats)
		}
		if offset > len(stats) {
			stats = nil
		} else {
			stats = stats[offset:end]
		}
	}

	return stats, totalCount, nil
}

func (e *taskRepository) FindByTaskName(ctx context.Context, projectId uuid.UUID, taskName string, fromDate, toDate time.Time, page, pageSize int, orderBy string, sortDirection string) ([]models.Task, int64, error) {
	params := lit.P{"project_id": projectId, "task_name": taskName, "from": fromDate.UTC(), "to": toDate.UTC()}

	countResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM tasks WHERE project_id = :project_id AND task_name = :task_name AND recorded_at >= :from AND recorded_at <= :to",
		params)
	if err != nil {
		return nil, 0, err
	}
	count := int64(0)
	if countResult != nil {
		count = int64(countResult.Count)
	}

	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{"recorded_at": true, "duration": true}
	if !allowedOrderBy[orderBy] {
		orderBy = "recorded_at"
	}

	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	rows, err := lit.SelectNamed[task](db.TelemetryDB,
		fmt.Sprintf(`SELECT id, project_id, task_name, duration, recorded_at, client_ip, attributes, app_version, server_name, distributed_trace_id
		FROM tasks WHERE project_id = :project_id AND task_name = :task_name AND recorded_at >= :from AND recorded_at <= :to
		ORDER BY %s %s LIMIT :limit OFFSET :offset`, orderBy, sortDir),
		lit.P{"project_id": projectId, "task_name": taskName, "from": fromDate.UTC(), "to": toDate.UTC(), "limit": pageSize, "offset": offset})
	if err != nil {
		return nil, 0, err
	}

	tasks := make([]models.Task, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, row.toModel())
	}

	return tasks, count, nil
}

func (e *taskRepository) FindById(ctx context.Context, projectId, taskId uuid.UUID, recordedAt *time.Time) (*models.Task, error) {
	query := `SELECT id, project_id, task_name, duration, recorded_at, client_ip, attributes, app_version, server_name, distributed_trace_id, span_id, is_root
		FROM tasks WHERE project_id = :project_id AND id = :id`
	params := lit.P{"project_id": projectId, "id": taskId}
	if recordedAt != nil {
		from, to := shared.TraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = from.UTC()
		params["to"] = to.UTC()
	}
	query += ` LIMIT 1`

	row, err := lit.SelectSingleNamed[task](db.TelemetryDB, query, params)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	t := row.toModel()
	return &t, nil
}

func (e *taskRepository) CountByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	return queryTaskTimeSeries(ctx,
		`SELECT `+timeBucketExpr("recorded_at", 3600)+` as bucket, CAST(COUNT(*) AS DOUBLE) as agg_value
		FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()})
}

func (e *taskRepository) AvgDurationByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	return queryTaskTimeSeries(ctx,
		`SELECT `+timeBucketExpr("recorded_at", 3600)+` as bucket, AVG(duration) / 1000000.0 as agg_value
		FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()})
}

func (e *taskRepository) CountByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	return queryTaskTimeSeries(ctx,
		`SELECT `+timeBucketExpr("recorded_at", intervalMinutes*60)+` as bucket, CAST(COUNT(*) AS DOUBLE) as agg_value
		FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()})
}

func (e *taskRepository) AvgDurationByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	return queryTaskTimeSeries(ctx,
		`SELECT `+timeBucketExpr("recorded_at", intervalMinutes*60)+` as bucket, AVG(duration) / 1000000.0 as agg_value
		FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()})
}

func (e *taskRepository) FindWorstTasks(ctx context.Context, projectId uuid.UUID, start, end time.Time, limit int) ([]models.TaskStats, error) {
	params := lit.P{"project_id": projectId, "from": start.UTC(), "to": end.UTC()}

	groups, err := lit.SelectNamed[taskGroupRow](db.TelemetryDB,
		`SELECT task_name, COUNT(*) as count, AVG(duration) as avg_duration, MAX(recorded_at) as last_seen,
		quantile_cont(duration, 0.5) as p50, quantile_cont(duration, 0.95) as p95
		FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY task_name`,
		params)
	if err != nil {
		return nil, err
	}

	var stats []models.TaskStats
	for _, g := range groups {
		stats = append(stats, models.TaskStats{
			TaskName:    g.TaskName,
			Count:       g.Count,
			P50Duration: time.Duration(g.P50),
			P95Duration: time.Duration(g.P95),
			AvgDuration: time.Duration(g.AvgDuration),
			LastSeen:    g.LastSeen,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		impactI := float64(stats[i].Count) * float64(stats[i].P95Duration-stats[i].P50Duration)
		impactJ := float64(stats[j].Count) * float64(stats[j].P95Duration-stats[j].P50Duration)
		return impactI > impactJ
	})

	if limit > len(stats) {
		limit = len(stats)
	}
	return stats[:limit], nil
}

func (e *taskRepository) GetTaskStats(ctx context.Context, projectId uuid.UUID, taskName string, start, end time.Time) (*models.TaskDetailStats, error) {
	params := lit.P{"project_id": projectId, "task_name": taskName, "from": start.UTC(), "to": end.UTC()}

	durationMinutes := end.Sub(start).Minutes()
	if durationMinutes < 1 {
		durationMinutes = 1
	}

	statsRow, err := lit.SelectSingleNamed[taskCountStatsRow](db.TelemetryDB,
		"SELECT COUNT(*) AS count, CASE WHEN COUNT(*) > 0 THEN AVG(duration) / 1000000.0 ELSE 0 END AS avg_dur_ms FROM tasks WHERE project_id = :project_id AND task_name = :task_name AND recorded_at >= :from AND recorded_at <= :to",
		params)
	if err != nil {
		return nil, err
	}
	if statsRow == nil {
		return &models.TaskDetailStats{}, nil
	}

	pctQuery, pctArgs, err := lit.ParseNamedQuery(db.Driver,
		`SELECT quantile_cont(duration, 0.5) AS p50, quantile_cont(duration, 0.95) AS p95, quantile_cont(duration, 0.99) AS p99
		FROM tasks WHERE project_id = :project_id AND task_name = :task_name AND recorded_at >= :from AND recorded_at <= :to`,
		params)
	if err != nil {
		return nil, err
	}

	var p50, p95, p99 sql.NullFloat64
	if err := db.TelemetryDB.QueryRowContext(ctx, pctQuery, pctArgs...).Scan(&p50, &p95, &p99); err != nil {
		return nil, err
	}

	nsToMs := 1000000.0

	return &models.TaskDetailStats{
		Count:          statsRow.Count,
		AvgDuration:    statsRow.AvgDurMs,
		MedianDuration: p50.Float64 / nsToMs,
		P95Duration:    p95.Float64 / nsToMs,
		P99Duration:    p99.Float64 / nsToMs,
		Throughput:     float64(statsRow.Count) / durationMinutes,
	}, nil
}

func (e *taskRepository) FindByDistributedTraceId(ctx context.Context, distributedTraceId uuid.UUID, projectIds []uuid.UUID, recordedAt *time.Time) ([]models.Task, error) {
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
	query := `SELECT id, project_id, task_name, duration, recorded_at, client_ip, attributes, app_version, server_name, distributed_trace_id
		FROM tasks WHERE distributed_trace_id = :trace_id AND project_id IN (` + strings.Join(placeholders, ",") + `)`
	if recordedAt != nil {
		from, to := shared.DistributedTraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = from.UTC()
		params["to"] = to.UTC()
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

	var tasks []models.Task
	for sqlRows.Next() {
		var row task
		if err := sqlRows.Scan(&row.Id, &row.ProjectId, &row.TaskName, &row.Duration, &row.RecordedAt, &row.ClientIP, &row.Attributes, &row.AppVersion, &row.ServerName, &row.DistributedTraceId); err != nil {
			return nil, err
		}
		tasks = append(tasks, row.toModel())
	}
	return tasks, nil
}

// queryTaskTimeSeries runs a time-bucketed aggregation and scans the bucket as a
// native TIMESTAMP (time.Time) — DuckDB returns TIMESTAMP columns directly as
// time.Time, so no string parsing of the bucket is needed (unlike the SQLite path).
func queryTaskTimeSeries(ctx context.Context, query string, params lit.P) ([]models.TimeSeriesPoint, error) {
	parsedQuery, args, err := lit.ParseNamedQuery(db.Driver, query, params)
	if err != nil {
		return nil, err
	}

	rows, err := db.TelemetryDB.QueryContext(ctx, parsedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := make([]models.TimeSeriesPoint, 0)
	for rows.Next() {
		var bucket time.Time
		var value float64
		if err := rows.Scan(&bucket, &value); err != nil {
			return nil, err
		}
		points = append(points, models.TimeSeriesPoint{Timestamp: bucket, Value: value})
	}
	return points, nil
}

var TaskRepository = &taskRepository{}
