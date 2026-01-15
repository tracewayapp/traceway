package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
	"encoding/json"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type taskRepository struct{}

func (e *taskRepository) InsertAsync(ctx context.Context, lines []models.Task) error {
	batch, err := (*chdb.Conn).PrepareBatch(clickhouse.Context(context.Background(), clickhouse.WithAsync(false)), "INSERT INTO tasks (id, project_id, task_name, duration, recorded_at, client_ip, scope, app_version, server_name)")
	if err != nil {
		return err
	}
	for _, t := range lines {
		scopeJSON := "{}"
		if len(t.Scope) != 0 {
			if scopeBytes, err := json.Marshal(t.Scope); err == nil {
				scopeJSON = string(scopeBytes)
			}
		}
		if err := batch.Append(t.Id, t.ProjectId, t.TaskName, t.Duration, t.RecordedAt, t.ClientIP, scopeJSON, t.AppVersion, t.ServerName); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (e *taskRepository) CountBetween(ctx context.Context, projectId string, start, end time.Time) (int64, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT count() FROM tasks WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, start, end).Scan(&count)
	return int64(count), err
}

func (e *taskRepository) FindAll(ctx context.Context, projectId string, fromDate, toDate time.Time, page, pageSize int, orderBy string) ([]models.Task, int64, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT count() FROM tasks WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, fromDate, toDate).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{
		"recorded_at": true,
		"duration":    true,
	}

	if !allowedOrderBy[orderBy] {
		orderBy = "recorded_at"
	}

	query := "SELECT id, project_id, task_name, duration, recorded_at, client_ip, scope, app_version, server_name FROM tasks WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY " + orderBy + " DESC LIMIT ? OFFSET ?"
	rows, err := (*chdb.Conn).Query(ctx, query, projectId, fromDate, toDate, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		var scopeJSON string
		if err := rows.Scan(&t.Id, &t.ProjectId, &t.TaskName, &t.Duration, &t.RecordedAt, &t.ClientIP, &scopeJSON, &t.AppVersion, &t.ServerName); err != nil {
			return nil, 0, err
		}
		// Parse scope JSON
		if scopeJSON != "" && scopeJSON != "{}" {
			if err := json.Unmarshal([]byte(scopeJSON), &t.Scope); err != nil {
				t.Scope = nil
			}
		}
		tasks = append(tasks, t)
	}

	return tasks, int64(count), nil
}

func (e *taskRepository) FindGroupedByTaskName(ctx context.Context, projectId string, fromDate, toDate time.Time, page, pageSize int, orderBy string, sortDirection string) ([]models.TaskStats, int64, error) {
	// Count unique task names
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT uniq(task_name) FROM tasks WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, fromDate, toDate).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize

	// Map frontend field names to SQL expressions
	orderByMap := map[string]string{
		"count":        "count",
		"p50_duration": "p50_duration",
		"p95_duration": "p95_duration",
		"avg_duration": "avg_duration",
		"last_seen":    "last_seen",
		"impact":       "count * (p95_duration - p50_duration)",
	}

	orderExpr, ok := orderByMap[orderBy]
	if !ok {
		orderExpr = orderByMap["impact"] // Default to impact expression
	}

	// Validate sort direction
	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	query := `SELECT
		task_name,
		count() as count,
		quantile(0.5)(duration) as p50_duration,
		quantile(0.95)(duration) as p95_duration,
		avg(duration) as avg_duration,
		max(recorded_at) as last_seen
	FROM tasks
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY task_name
	ORDER BY ` + orderExpr + ` ` + sortDir + `
	LIMIT ? OFFSET ?`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, fromDate, toDate, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var stats []models.TaskStats
	for rows.Next() {
		var s models.TaskStats
		var p50, p95, avg float64
		if err := rows.Scan(&s.TaskName, &s.Count, &p50, &p95, &avg, &s.LastSeen); err != nil {
			return nil, 0, err
		}
		s.P50Duration = time.Duration(p50)
		s.P95Duration = time.Duration(p95)
		s.AvgDuration = time.Duration(avg)
		stats = append(stats, s)
	}

	return stats, int64(count), nil
}

func (e *taskRepository) FindByTaskName(ctx context.Context, projectId string, taskName string, fromDate, toDate time.Time, page, pageSize int, orderBy string, sortDirection string) ([]models.Task, int64, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT count() FROM tasks WHERE project_id = ? AND task_name = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, taskName, fromDate, toDate).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{
		"recorded_at": true,
		"duration":    true,
	}

	if !allowedOrderBy[orderBy] {
		orderBy = "recorded_at"
	}

	// Validate sort direction
	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	query := "SELECT id, project_id, task_name, duration, recorded_at, client_ip, scope, app_version, server_name FROM tasks WHERE project_id = ? AND task_name = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY " + orderBy + " " + sortDir + " LIMIT ? OFFSET ?"
	rows, err := (*chdb.Conn).Query(ctx, query, projectId, taskName, fromDate, toDate, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		var scopeJSON string
		if err := rows.Scan(&t.Id, &t.ProjectId, &t.TaskName, &t.Duration, &t.RecordedAt, &t.ClientIP, &scopeJSON, &t.AppVersion, &t.ServerName); err != nil {
			return nil, 0, err
		}
		// Parse scope JSON
		if scopeJSON != "" && scopeJSON != "{}" {
			if err := json.Unmarshal([]byte(scopeJSON), &t.Scope); err != nil {
				t.Scope = nil
			}
		}
		tasks = append(tasks, t)
	}

	return tasks, int64(count), nil
}

// FindById returns a single task by ID
func (e *taskRepository) FindById(ctx context.Context, projectId, taskId string) (*models.Task, error) {
	query := `SELECT id, project_id, task_name, duration, recorded_at, client_ip, scope, app_version, server_name
		FROM tasks
		WHERE project_id = ? AND id = ?
		LIMIT 1`

	var t models.Task
	var scopeJSON string

	err := (*chdb.Conn).QueryRow(ctx, query, projectId, taskId).Scan(
		&t.Id, &t.ProjectId, &t.TaskName, &t.Duration, &t.RecordedAt,
		&t.ClientIP, &scopeJSON, &t.AppVersion, &t.ServerName)

	if err != nil {
		return nil, err
	}

	// Parse scope JSON
	if scopeJSON != "" && scopeJSON != "{}" {
		if err := json.Unmarshal([]byte(scopeJSON), &t.Scope); err != nil {
			t.Scope = nil
		}
	}

	return &t, nil
}

// CountByHour returns task counts grouped by hour
func (e *taskRepository) CountByHour(ctx context.Context, projectId string, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfHour(recorded_at) as hour,
		toFloat64(count()) as count
	FROM tasks
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY hour
	ORDER BY hour ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.TimeSeriesPoint
	for rows.Next() {
		var p models.TimeSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	return points, nil
}

// AvgDurationByHour returns average duration in ms grouped by hour
func (e *taskRepository) AvgDurationByHour(ctx context.Context, projectId string, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfHour(recorded_at) as hour,
		avg(duration) / 1000000 as avg_duration_ms
	FROM tasks
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY hour
	ORDER BY hour ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.TimeSeriesPoint
	for rows.Next() {
		var p models.TimeSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	return points, nil
}

// CountByInterval returns task counts grouped by configurable interval in minutes
func (e *taskRepository) CountByInterval(ctx context.Context, projectId string, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfInterval(recorded_at, INTERVAL ? MINUTE) as bucket,
		toFloat64(count()) as count
	FROM tasks
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY bucket
	ORDER BY bucket ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, intervalMinutes, projectId, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.TimeSeriesPoint
	for rows.Next() {
		var p models.TimeSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	return points, nil
}

// AvgDurationByInterval returns average duration in ms grouped by configurable interval
func (e *taskRepository) AvgDurationByInterval(ctx context.Context, projectId string, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfInterval(recorded_at, INTERVAL ? MINUTE) as bucket,
		avg(duration) / 1000000 as avg_duration_ms
	FROM tasks
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY bucket
	ORDER BY bucket ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, intervalMinutes, projectId, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.TimeSeriesPoint
	for rows.Next() {
		var p models.TimeSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	return points, nil
}

// FindWorstTasks returns tasks ordered by impact score (count * variance)
func (e *taskRepository) FindWorstTasks(ctx context.Context, projectId string, start, end time.Time, limit int) ([]models.TaskStats, error) {
	query := `SELECT
		task_name,
		count() as count,
		quantile(0.5)(duration) as p50_duration,
		quantile(0.95)(duration) as p95_duration,
		avg(duration) as avg_duration,
		max(recorded_at) as last_seen
	FROM tasks
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY task_name
	ORDER BY count * (p95_duration - p50_duration) DESC
	LIMIT ?`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.TaskStats
	for rows.Next() {
		var s models.TaskStats
		var p50, p95, avg float64
		if err := rows.Scan(&s.TaskName, &s.Count, &p50, &p95, &avg, &s.LastSeen); err != nil {
			return nil, err
		}
		s.P50Duration = time.Duration(p50)
		s.P95Duration = time.Duration(p95)
		s.AvgDuration = time.Duration(avg)
		stats = append(stats, s)
	}

	return stats, nil
}

// GetTaskStats returns aggregate statistics for a specific task
func (e *taskRepository) GetTaskStats(ctx context.Context, projectId, taskName string, start, end time.Time) (*models.TaskDetailStats, error) {
	// Calculate time range duration for throughput calculation
	durationMinutes := end.Sub(start).Minutes()
	if durationMinutes < 1 {
		durationMinutes = 1
	}

	query := `SELECT
		count() as count,
		avg(duration) / 1000000 as avg_duration_ms,
		quantile(0.5)(duration) / 1000000 as p50_duration_ms,
		quantile(0.95)(duration) / 1000000 as p95_duration_ms,
		quantile(0.99)(duration) / 1000000 as p99_duration_ms
	FROM tasks
	WHERE project_id = ? AND task_name = ? AND recorded_at >= ? AND recorded_at <= ?`

	var stats models.TaskDetailStats
	var count uint64

	err := (*chdb.Conn).QueryRow(ctx, query, projectId, taskName, start, end).Scan(
		&count,
		&stats.AvgDuration,
		&stats.MedianDuration,
		&stats.P95Duration,
		&stats.P99Duration,
	)
	if err != nil {
		return nil, err
	}

	stats.Count = int64(count)
	// Calculate throughput (tasks per minute)
	stats.Throughput = float64(count) / durationMinutes

	return &stats, nil
}

var TaskRepository = taskRepository{}
