//go:build !telemetry_ch && !telemetry_duckdb

package sqlite

import (
	"context"
	"time"

	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type checkResultRow struct {
	ProjectId        uuid.UUID              `lit:"project_id"`
	CheckId          int                    `lit:"check_id"`
	RunId            int                    `lit:"run_id"`
	CheckType        string                 `lit:"check_type"`
	RecordedAt       sqlitetypes.SQLiteTime `lit:"recorded_at"`
	ExecutedAt       sqlitetypes.SQLiteTime `lit:"executed_at"`
	Status           string                 `lit:"status"`
	LatencyMs        float64                `lit:"latency_ms"`
	StatusCode       int                    `lit:"status_code"`
	ErrorMsg         string                 `lit:"error_message"`
	ExecutedBy       string                 `lit:"executed_by"`
	ScreenshotKey    string                 `lit:"screenshot_key"`
	OutputKey        string                 `lit:"output_key"`
	TlsDaysRemaining int                    `lit:"tls_days_remaining"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[checkResultRow](driver)
	})
}

type checkResultRepository struct{}

func (r *checkResultRepository) Insert(ctx context.Context, result shared.CheckResult) error {
	query, args, err := lit.ParseNamedQuery(db.Driver,
		`INSERT INTO check_results (project_id, check_id, run_id, check_type, recorded_at, executed_at, status, latency_ms, status_code, error_message, executed_by, screenshot_key, output_key, tls_days_remaining)
		VALUES (:project_id, :check_id, :run_id, :check_type, :recorded_at, :executed_at, :status, :latency_ms, :status_code, :error_message, :executed_by, :screenshot_key, :output_key, :tls_days_remaining)`,
		lit.P{
			"project_id":         result.ProjectId,
			"check_id":           result.CheckId,
			"run_id":             result.RunId,
			"check_type":         result.CheckType,
			"recorded_at":        sqlitetypes.NewSQLiteTime(result.RecordedAt),
			"executed_at":        sqlitetypes.NewSQLiteTime(result.ExecutedAt),
			"status":             result.Status,
			"latency_ms":         result.LatencyMs,
			"status_code":        result.StatusCode,
			"error_message":      result.ErrorMsg,
			"executed_by":        result.ExecutedBy,
			"screenshot_key":     result.ScreenshotKey,
			"output_key":         result.OutputKey,
			"tls_days_remaining": result.TlsDaysRemaining,
		})
	if err != nil {
		return err
	}
	_, err = db.TelemetryDB.ExecContext(ctx, query, args...)
	return err
}

func (r *checkResultRepository) FindByCheck(ctx context.Context, projectId uuid.UUID, checkId int, from *time.Time, to *time.Time, status string, page int, pageSize int) ([]*models.CheckResultEntry, int64, error) {
	where := "WHERE project_id = ? AND check_id = ?"
	args := []interface{}{projectId.String(), checkId}

	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	if from != nil {
		where += " AND recorded_at >= ?"
		args = append(args, from.UTC().Format(time.RFC3339Nano))
	}
	if to != nil {
		where += " AND recorded_at <= ?"
		args = append(args, to.UTC().Format(time.RFC3339Nano))
	}

	var total int64
	if err := db.TelemetryDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM check_results "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT run_id, check_type, recorded_at, executed_at, status, latency_ms, status_code, error_message, executed_by, screenshot_key, output_key, tls_days_remaining FROM check_results " +
		where + " ORDER BY recorded_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*models.CheckResultEntry
	for rows.Next() {
		entry := &models.CheckResultEntry{}
		var recordedAt, executedAt sqlitetypes.SQLiteTime
		if err := rows.Scan(
			&entry.RunId, &entry.CheckType, &recordedAt, &executedAt, &entry.Status,
			&entry.LatencyMs, &entry.StatusCode, &entry.ErrorMessage, &entry.ExecutedBy,
			&entry.ScreenshotKey, &entry.OutputKey, &entry.TlsDaysRemaining,
		); err != nil {
			return nil, 0, err
		}
		entry.RecordedAt = recordedAt.Time
		entry.ExecutedAt = executedAt.Time
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// AggregatesByProject returns per-check result summaries for the range.
func (r *checkResultRepository) AggregatesByProject(ctx context.Context, projectId uuid.UUID, from time.Time, to time.Time) (map[int]*models.CheckAggregates, error) {
	query := `SELECT check_id,
		COUNT(*),
		SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END),
		SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END),
		SUM(CASE WHEN status = 'missed' THEN 1 ELSE 0 END),
		COALESCE(AVG(CASE WHEN status <> 'missed' THEN latency_ms END), 0)
	FROM check_results
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY check_id`

	rows, err := db.TelemetryDB.QueryContext(ctx, query, projectId.String(), from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]*models.CheckAggregates)
	for rows.Next() {
		var checkId int
		agg := &models.CheckAggregates{}
		if err := rows.Scan(&checkId, &agg.Total, &agg.Up, &agg.Down, &agg.Missed, &agg.AvgLatencyMs); err != nil {
			return nil, err
		}
		result[checkId] = agg
	}
	return result, rows.Err()
}

// SeriesByCheck buckets one check's results for the latency chart and uptime
// bars. Buckets are epoch-floored to align with the other backends.
func (r *checkResultRepository) SeriesByCheck(ctx context.Context, projectId uuid.UUID, checkId int, from time.Time, to time.Time, intervalMinutes int) ([]*models.CheckSeriesPoint, error) {
	bucketSeconds := intervalMinutes * 60
	query := `SELECT (CAST(strftime('%s', recorded_at) AS INTEGER) / ?) * ? AS bucket,
		COUNT(*),
		SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END),
		SUM(CASE WHEN status = 'missed' THEN 1 ELSE 0 END),
		COALESCE(AVG(CASE WHEN status <> 'missed' THEN latency_ms END), 0),
		COALESCE(MAX(CASE WHEN status <> 'missed' THEN latency_ms END), 0)
	FROM check_results
	WHERE project_id = ? AND check_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY bucket ORDER BY bucket ASC`

	rows, err := db.TelemetryDB.QueryContext(ctx, query, bucketSeconds, bucketSeconds, projectId.String(), checkId, from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []*models.CheckSeriesPoint
	for rows.Next() {
		var bucket int64
		point := &models.CheckSeriesPoint{}
		if err := rows.Scan(&bucket, &point.Total, &point.Up, &point.Missed, &point.AvgLatencyMs, &point.MaxLatencyMs); err != nil {
			return nil, err
		}
		point.Bucket = time.Unix(bucket, 0).UTC()
		points = append(points, point)
	}
	return points, rows.Err()
}

var CheckResultRepository = &checkResultRepository{}
