//go:build !pgch

package repositories

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type task struct {
	Id                 uuid.UUID     `lit:"id"`
	ProjectId          uuid.UUID     `lit:"project_id"`
	TaskName           string        `lit:"task_name"`
	Duration           int64         `lit:"duration"`
	RecordedAt         SQLiteTime    `lit:"recorded_at"`
	ClientIP           string        `lit:"client_ip"`
	Attributes         SQLiteJSONMap `lit:"attributes"`
	AppVersion         string        `lit:"app_version"`
	ServerName         string        `lit:"server_name"`
	DistributedTraceId *uuid.UUID    `lit:"distributed_trace_id"`
	SpanId             *uuid.UUID    `lit:"span_id"`
	IsRoot             bool          `lit:"is_root"`
}

type taskGroupRow struct {
	TaskName    string  `lit:"task_name"`
	Count       uint64  `lit:"count"`
	AvgDuration float64 `lit:"avg_duration"`
	LastSeen    string  `lit:"last_seen"`
	HasRoot     bool    `lit:"has_root"`
	HasNonRoot  bool    `lit:"has_non_root"`
}

type taskCountStatsRow struct {
	Count       int64   `lit:"count"`
	AvgDurMs    float64 `lit:"avg_dur_ms"`
}

type durationValueRow struct {
	Duration float64 `lit:"duration"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[task](driver)
		lit.RegisterModel[taskGroupRow](driver)
		lit.RegisterModel[taskCountStatsRow](driver)
		lit.RegisterModel[durationValueRow](driver)
	})
}

func taskToRow(t models.Task) task {
	return task{
		Id:                 t.Id,
		ProjectId:          t.ProjectId,
		TaskName:           t.TaskName,
		Duration:           int64(t.Duration),
		RecordedAt:         NewSQLiteTime(t.RecordedAt),
		ClientIP:           t.ClientIP,
		Attributes:         NewSQLiteJSONMap(t.Attributes),
		AppVersion:         t.AppVersion,
		ServerName:         t.ServerName,
		DistributedTraceId: t.DistributedTraceId,
		SpanId:             t.SpanId,
		IsRoot:             t.IsRoot,
	}
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

func (e *taskRepository) InsertAsync(ctx context.Context, lines []models.Task) error {
	if len(lines) == 0 {
		return nil
	}

	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, t := range lines {
		row := taskToRow(t)
		if err := lit.InsertExistingUuid(tx, &row); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (e *taskRepository) CountBetween(ctx context.Context, projectId uuid.UUID, start, end time.Time) (int64, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(*) AS count FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to",
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
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
		lit.P{"project_id": projectId, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate)})
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
		lit.P{"project_id": projectId, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate), "limit": pageSize, "offset": offset})
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
	params := lit.P{"project_id": projectId, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate)}
	whereClause := "project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to"
	if search != "" {
		whereClause += " AND INSTR(LOWER(task_name), LOWER(:search)) > 0"
		params["search"] = search
	}
	whereClause += rootFilterClause("is_root", rootFilter)

	totalResult, err := lit.SelectSingleNamed[models.CountResult](db.TelemetryDB,
		"SELECT COUNT(DISTINCT task_name) AS count FROM tasks WHERE "+whereClause,
		params)
	if err != nil {
		return nil, 0, err
	}
	totalCount := int64(0)
	if totalResult != nil {
		totalCount = int64(totalResult.Count)
	}

	needsGoSort := orderBy == "p50_duration" || orderBy == "p95_duration" || orderBy == "impact"

	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	offset := (page - 1) * pageSize

	var baseQuery string
	groupParams := lit.P{"project_id": projectId, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate)}
	if search != "" {
		groupParams["search"] = search
	}

	groupedCols := `task_name, COUNT(*) as count, AVG(duration) as avg_duration, MAX(recorded_at) as last_seen,
			MAX(is_root) as has_root, MAX(CASE WHEN is_root = 0 THEN 1 ELSE 0 END) as has_non_root`

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
		groupParams["limit"] = pageSize
		groupParams["offset"] = offset
	}

	groups, err := lit.SelectNamed[taskGroupRow](db.TelemetryDB, baseQuery, groupParams)
	if err != nil {
		return nil, 0, err
	}

	var stats []models.TaskStats
	for _, g := range groups {
		durations, err := fetchSortedTaskDurations(ctx, projectId, g.TaskName, fromDate, toDate)
		if err != nil {
			return nil, 0, err
		}

		ls, _ := time.Parse(time.RFC3339Nano, g.LastSeen)
		stats = append(stats, models.TaskStats{
			TaskName:    g.TaskName,
			Count:       g.Count,
			P50Duration: time.Duration(computePercentile(durations, 0.5)),
			P95Duration: time.Duration(computePercentile(durations, 0.95)),
			AvgDuration: time.Duration(g.AvgDuration),
			LastSeen:    ls,
			HasRoot:     g.HasRoot,
			HasNonRoot:  g.HasNonRoot,
		})
	}

	if needsGoSort {
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
	params := lit.P{"project_id": projectId, "task_name": taskName, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate)}

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
		lit.P{"project_id": projectId, "task_name": taskName, "from": NewSQLiteTime(fromDate), "to": NewSQLiteTime(toDate), "limit": pageSize, "offset": offset})
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
		from, to := traceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = NewSQLiteTime(from)
		params["to"] = NewSQLiteTime(to)
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
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		`SELECT strftime('%Y-%m-%d %H:00:00', recorded_at) as bucket, CAST(COUNT(*) AS REAL) as agg_value
		FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *taskRepository) AvgDurationByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		`SELECT strftime('%Y-%m-%d %H:00:00', recorded_at) as bucket, AVG(duration) / 1000000.0 as agg_value
		FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`,
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *taskRepository) CountByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		fmt.Sprintf(`SELECT datetime((strftime('%%s', recorded_at) / %d) * %d, 'unixepoch') as bucket, CAST(COUNT(*) AS REAL) as agg_value
		FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`, secs, secs),
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *taskRepository) AvgDurationByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	secs := intervalMinutes * 60
	results, err := lit.SelectNamed[timeSeriesResult](db.TelemetryDB,
		fmt.Sprintf(`SELECT datetime((strftime('%%s', recorded_at) / %d) * %d, 'unixepoch') as bucket, AVG(duration) / 1000000.0 as agg_value
		FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY bucket ORDER BY bucket ASC`, secs, secs),
		lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)})
	if err != nil {
		return nil, err
	}
	return timeSeriesResultsToPoints(results), nil
}

func (e *taskRepository) FindWorstTasks(ctx context.Context, projectId uuid.UUID, start, end time.Time, limit int) ([]models.TaskStats, error) {
	params := lit.P{"project_id": projectId, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)}

	groups, err := lit.SelectNamed[taskGroupRow](db.TelemetryDB,
		`SELECT task_name, COUNT(*) as count, AVG(duration) as avg_duration, MAX(recorded_at) as last_seen
		FROM tasks WHERE project_id = :project_id AND recorded_at >= :from AND recorded_at <= :to
		GROUP BY task_name`,
		params)
	if err != nil {
		return nil, err
	}

	var stats []models.TaskStats
	for _, g := range groups {
		durations, err := fetchSortedTaskDurations(ctx, projectId, g.TaskName, start, end)
		if err != nil {
			return nil, err
		}

		ls, _ := time.Parse(time.RFC3339Nano, g.LastSeen)
		stats = append(stats, models.TaskStats{
			TaskName:    g.TaskName,
			Count:       g.Count,
			P50Duration: time.Duration(computePercentile(durations, 0.5)),
			P95Duration: time.Duration(computePercentile(durations, 0.95)),
			AvgDuration: time.Duration(g.AvgDuration),
			LastSeen:    ls,
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
	params := lit.P{"project_id": projectId, "task_name": taskName, "from": NewSQLiteTime(start), "to": NewSQLiteTime(end)}

	durationMinutes := end.Sub(start).Minutes()
	if durationMinutes < 1 {
		durationMinutes = 1
	}

	statsRow, err := lit.SelectSingleNamed[taskCountStatsRow](db.TelemetryDB,
		"SELECT COUNT(*) AS count, AVG(duration) / 1000000.0 AS avg_dur_ms FROM tasks WHERE project_id = :project_id AND task_name = :task_name AND recorded_at >= :from AND recorded_at <= :to",
		params)
	if err != nil {
		return nil, err
	}
	if statsRow == nil {
		return &models.TaskDetailStats{}, nil
	}

	durations, err := fetchSortedTaskDurations(ctx, projectId, taskName, start, end)
	if err != nil {
		return nil, err
	}

	nsToMs := 1000000.0

	return &models.TaskDetailStats{
		Count:          statsRow.Count,
		AvgDuration:    statsRow.AvgDurMs,
		MedianDuration: computePercentile(durations, 0.5) / nsToMs,
		P95Duration:    computePercentile(durations, 0.95) / nsToMs,
		P99Duration:    computePercentile(durations, 0.99) / nsToMs,
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
		from, to := distributedTraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = NewSQLiteTime(from)
		params["to"] = NewSQLiteTime(to)
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

func fetchSortedTaskDurations(ctx context.Context, projectId uuid.UUID, taskName string, from, to time.Time) ([]float64, error) {
	results, err := lit.SelectNamed[durationValueRow](db.TelemetryDB,
		"SELECT duration FROM tasks WHERE project_id = :project_id AND task_name = :task_name AND recorded_at >= :from AND recorded_at <= :to ORDER BY duration ASC",
		lit.P{"project_id": projectId, "task_name": taskName, "from": NewSQLiteTime(from), "to": NewSQLiteTime(to)})
	if err != nil {
		return nil, err
	}
	durations := make([]float64, 0, len(results))
	for _, r := range results {
		durations = append(durations, r.Duration)
	}
	return durations, nil
}

var TaskRepository = taskRepository{}
