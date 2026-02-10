package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

type endpointRepository struct{}

func (e *endpointRepository) InsertAsync(ctx context.Context, lines []models.Endpoint) error {
	batch, err := (*chdb.Conn).PrepareBatch(clickhouse.Context(context.Background(), clickhouse.WithAsync(false)), "INSERT INTO endpoints (id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name)")
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
		if err := batch.Append(t.Id, t.ProjectId, t.Endpoint, t.Duration, t.RecordedAt, t.StatusCode, t.BodySize, t.ClientIP, attributesJSON, t.AppVersion, t.ServerName); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (e *endpointRepository) CountBetween(ctx context.Context, projectId uuid.UUID, start, end time.Time) (int64, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT count() FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, start, end).Scan(&count)
	return int64(count), err
}

func (e *endpointRepository) FindAll(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy string) ([]models.Endpoint, int64, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT count() FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, fromDate, toDate).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{
		"recorded_at": true,
		"duration":    true,
		"status_code": true,
		"body_size":   true,
	}

	if !allowedOrderBy[orderBy] {
		orderBy = "recorded_at"
	}

	query := "SELECT id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name FROM endpoints WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY " + orderBy + " DESC LIMIT ? OFFSET ?"
	rows, err := (*chdb.Conn).Query(ctx, query, projectId, fromDate, toDate, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var endpoints []models.Endpoint
	for rows.Next() {
		var t models.Endpoint
		var attributesJSON string
		if err := rows.Scan(&t.Id, &t.ProjectId, &t.Endpoint, &t.Duration, &t.RecordedAt, &t.StatusCode, &t.BodySize, &t.ClientIP, &attributesJSON, &t.AppVersion, &t.ServerName); err != nil {
			return nil, 0, err
		}
		if attributesJSON != "" && attributesJSON != "{}" {
			if err := json.Unmarshal([]byte(attributesJSON), &t.Attributes); err != nil {
				t.Attributes = nil
			}
		}
		endpoints = append(endpoints, t)
	}

	return endpoints, int64(count), nil
}

func (e *endpointRepository) FindGroupedByEndpoint(ctx context.Context, projectId uuid.UUID, fromDate, toDate time.Time, page, pageSize int, orderBy string, sortDirection string, search string) ([]models.EndpointStats, int64, error) {
	// Build WHERE clause with optional search filter
	// Count query uses bare column names; main query uses e. prefix for LEFT JOIN
	whereClause := "project_id = ? AND recorded_at >= ? AND recorded_at <= ?"
	joinWhereClause := "e.project_id = ? AND e.recorded_at >= ? AND e.recorded_at <= ?"
	args := []interface{}{projectId, fromDate, toDate}

	if search != "" {
		whereClause += " AND positionCaseInsensitive(endpoint, ?) > 0"
		joinWhereClause += " AND positionCaseInsensitive(e.endpoint, ?) > 0"
		args = append(args, search)
	}

	// Count unique endpoints
	var count uint64
	countQuery := "SELECT uniq(endpoint) FROM endpoints WHERE " + whereClause
	err := (*chdb.Conn).QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize

	// Map frontend field names to SQL expressions
	orderByMap := map[string]string{
		"count":        "count",
		"p50_duration": "p50_duration",
		"p95_duration": "p95_duration",
		"p99_duration": "p99_duration",
		"avg_duration": "avg_duration",
		"last_seen":    "last_seen",
		"impact":       "impact",
	}

	orderExpr, ok := orderByMap[orderBy]
	if !ok {
		orderExpr = "impact" // Default to impact
	}

	// Validate sort direction
	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	query := `SELECT
		endpoint, total_count as count, p50_duration, p95_duration, p99_duration,
		avg_duration, last_seen, offset_ms,
		satisfied_count, tolerating_count, bad_count, client_error_count,
		greatest(
			if(total_count > 0,
				1.0 - ((satisfied_count + tolerating_count * 0.5) / total_count), 0.0),
			multiIf(
				bad_count / total_count > 0.33, 0.75,
				bad_count / total_count > 0.20, 0.50,
				bad_count / total_count > 0.10, 0.25, 0.0),
			multiIf(
				toFloat64(p99_duration) - toFloat64(offset_ms) * 1000000 > 8000000000, 0.75,
				toFloat64(p99_duration) - toFloat64(offset_ms) * 1000000 > 6000000000, 0.50,
				toFloat64(p99_duration) - toFloat64(offset_ms) * 1000000 > 3000000000, 0.25, 0.0),
			if(total_count > 10,
				multiIf(
					client_error_count / total_count > 0.50, 0.75,
					client_error_count / total_count > 0.25, 0.50, 0.0),
				0.0),
			multiIf(
				bad_count / total_count > 0.10 AND bad_count >= 500, 0.75,
				bad_count / total_count > 0.10 AND bad_count >= 50, 0.50,
				bad_count / total_count > 0.05 AND bad_count >= 2000, 0.75,
				bad_count / total_count > 0.05 AND bad_count >= 500, 0.50,
				bad_count / total_count > 0.05 AND bad_count >= 50, 0.25,
				bad_count / total_count > 0.01 AND bad_count >= 10000, 0.75,
				bad_count / total_count > 0.01 AND bad_count >= 2000, 0.50,
				bad_count / total_count > 0.01 AND bad_count >= 500, 0.25,
				0.0)
		) as impact
	FROM (
		SELECT
			endpoint,
			offset_ms,
			count() as total_count,
			quantile(0.5)(duration) as p50_duration,
			quantile(0.95)(duration) as p95_duration,
			quantile(0.99)(duration) as p99_duration,
			avg(duration) as avg_duration,
			max(recorded_at) as last_seen,
			countIf(duration <= (750000000 + toInt64(offset_ms) * 1000000)
				AND status_code < 500) as satisfied_count,
			countIf(duration > (750000000 + toInt64(offset_ms) * 1000000)
				AND duration <= (1500000000 + toInt64(offset_ms) * 1000000)
				AND status_code < 500) as tolerating_count,
			countIf(duration > (1500000000 + toInt64(offset_ms) * 1000000)
				OR status_code >= 500) as bad_count,
			countIf(status_code >= 400 AND status_code < 500) as client_error_count
		FROM (
			SELECT e.endpoint, e.duration, e.status_code, e.recorded_at,
				   s.offset_ms as offset_ms
			FROM endpoints e
			LEFT JOIN (SELECT * FROM slow_endpoints FINAL) AS s
				ON e.endpoint = s.endpoint AND e.project_id = s.project_id
			WHERE ` + joinWhereClause + `
		)
		GROUP BY endpoint, offset_ms
	)
	ORDER BY ` + orderExpr + ` ` + sortDir + `
	LIMIT ? OFFSET ?`

	// Add pagination args
	queryArgs := append(args, pageSize, offset)

	rows, err := (*chdb.Conn).Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var stats []models.EndpointStats
	for rows.Next() {
		var s models.EndpointStats
		var p50, p95, p99, avg float64
		var offsetMs uint32
		var satisfiedCount, toleratingCount, badCount, clientErrorCount uint64
		if err := rows.Scan(&s.Endpoint, &s.Count, &p50, &p95, &p99, &avg, &s.LastSeen,
			&offsetMs, &satisfiedCount, &toleratingCount, &badCount, &clientErrorCount,
			&s.Impact); err != nil {
			return nil, 0, err
		}
		s.P50Duration = time.Duration(p50)
		s.P95Duration = time.Duration(p95)
		s.P99Duration = time.Duration(p99)
		s.AvgDuration = time.Duration(avg)
		s.ImpactReason = computeImpactReason(s.Count, satisfiedCount, toleratingCount, badCount, clientErrorCount, p99, offsetMs)
		stats = append(stats, s)
	}

	return stats, int64(count), nil
}

func (e *endpointRepository) FindByEndpoint(ctx context.Context, projectId uuid.UUID, endpoint string, fromDate, toDate time.Time, page, pageSize int, orderBy string, sortDirection string) ([]models.Endpoint, int64, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT count() FROM endpoints WHERE project_id = ? AND endpoint = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, endpoint, fromDate, toDate).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{
		"recorded_at": true,
		"duration":    true,
		"status_code": true,
		"body_size":   true,
	}

	if !allowedOrderBy[orderBy] {
		orderBy = "recorded_at"
	}

	// Validate sort direction
	sortDir := "DESC"
	if sortDirection == "asc" {
		sortDir = "ASC"
	}

	query := "SELECT id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name FROM endpoints WHERE project_id = ? AND endpoint = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY " + orderBy + " " + sortDir + " LIMIT ? OFFSET ?"
	rows, err := (*chdb.Conn).Query(ctx, query, projectId, endpoint, fromDate, toDate, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var endpoints []models.Endpoint
	for rows.Next() {
		var t models.Endpoint
		var attributesJSON string
		if err := rows.Scan(&t.Id, &t.ProjectId, &t.Endpoint, &t.Duration, &t.RecordedAt, &t.StatusCode, &t.BodySize, &t.ClientIP, &attributesJSON, &t.AppVersion, &t.ServerName); err != nil {
			return nil, 0, err
		}
		if attributesJSON != "" && attributesJSON != "{}" {
			if err := json.Unmarshal([]byte(attributesJSON), &t.Attributes); err != nil {
				t.Attributes = nil
			}
		}
		endpoints = append(endpoints, t)
	}

	return endpoints, int64(count), nil
}

// FindById returns a single endpoint by ID
func (e *endpointRepository) FindById(ctx context.Context, projectId, endpointId uuid.UUID) (*models.Endpoint, error) {
	query := `SELECT id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name
		FROM endpoints
		WHERE project_id = ? AND id = ?
		LIMIT 1`

	var t models.Endpoint
	var attributesJSON string

	err := (*chdb.Conn).QueryRow(ctx, query, projectId, endpointId).Scan(
		&t.Id, &t.ProjectId, &t.Endpoint, &t.Duration, &t.RecordedAt,
		&t.StatusCode, &t.BodySize, &t.ClientIP, &attributesJSON, &t.AppVersion, &t.ServerName)

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

// CountByHour returns endpoint counts grouped by hour
func (e *endpointRepository) CountByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfHour(recorded_at) as hour,
		toFloat64(count()) as count
	FROM endpoints
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

// AvgDurationByHour returns average response time in ms grouped by hour
func (e *endpointRepository) AvgDurationByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfHour(recorded_at) as hour,
		avg(duration) / 1000000 as avg_duration_ms
	FROM endpoints
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

// ErrorRateByHour returns error rate (percentage) grouped by hour
func (e *endpointRepository) ErrorRateByHour(ctx context.Context, projectId uuid.UUID, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfHour(recorded_at) as hour,
		countIf(status_code >= 500) * 100.0 / count() as error_rate
	FROM endpoints
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

// CountByInterval returns endpoint counts grouped by configurable interval in minutes
func (e *endpointRepository) CountByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfInterval(recorded_at, INTERVAL ? MINUTE) as bucket,
		toFloat64(count()) as count
	FROM endpoints
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

// AvgDurationByInterval returns average response time in ms grouped by configurable interval
func (e *endpointRepository) AvgDurationByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfInterval(recorded_at, INTERVAL ? MINUTE) as bucket,
		avg(duration) / 1000000 as avg_duration_ms
	FROM endpoints
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

// ErrorRateByInterval returns error rate (percentage) grouped by configurable interval
func (e *endpointRepository) ErrorRateByInterval(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfInterval(recorded_at, INTERVAL ? MINUTE) as bucket,
		countIf(status_code >= 500) * 100.0 / count() as error_rate
	FROM endpoints
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

func (e *endpointRepository) FindWorstEndpoints(ctx context.Context, projectId uuid.UUID, start, end time.Time, limit int) ([]models.EndpointStats, error) {
	query := `SELECT
		endpoint, total_count, p50_duration, p95_duration, p99_duration,
		avg_duration, last_seen, offset_ms,
		satisfied_count, tolerating_count, bad_count, client_error_count,
		greatest(
			if(total_count > 0,
				1.0 - ((satisfied_count + tolerating_count * 0.5) / total_count), 0.0),
			multiIf(
				bad_count / total_count > 0.33, 0.75,
				bad_count / total_count > 0.20, 0.50,
				bad_count / total_count > 0.10, 0.25, 0.0),
			multiIf(
				toFloat64(p99_duration) - toFloat64(offset_ms) * 1000000 > 8000000000, 0.75,
				toFloat64(p99_duration) - toFloat64(offset_ms) * 1000000 > 6000000000, 0.50,
				toFloat64(p99_duration) - toFloat64(offset_ms) * 1000000 > 3000000000, 0.25, 0.0),
			if(total_count > 10,
				multiIf(
					client_error_count / total_count > 0.50, 0.75,
					client_error_count / total_count > 0.25, 0.50, 0.0),
				0.0),
			multiIf(
				bad_count / total_count > 0.10 AND bad_count >= 500, 0.75,
				bad_count / total_count > 0.10 AND bad_count >= 50, 0.50,
				bad_count / total_count > 0.05 AND bad_count >= 2000, 0.75,
				bad_count / total_count > 0.05 AND bad_count >= 500, 0.50,
				bad_count / total_count > 0.05 AND bad_count >= 50, 0.25,
				bad_count / total_count > 0.01 AND bad_count >= 10000, 0.75,
				bad_count / total_count > 0.01 AND bad_count >= 2000, 0.50,
				bad_count / total_count > 0.01 AND bad_count >= 500, 0.25,
				0.0)
		) as impact
	FROM (
		SELECT
			endpoint,
			offset_ms,
			count() as total_count,
			quantile(0.5)(duration) as p50_duration,
			quantile(0.95)(duration) as p95_duration,
			quantile(0.99)(duration) as p99_duration,
			avg(duration) as avg_duration,
			max(recorded_at) as last_seen,
			countIf(duration <= (750000000 + toInt64(offset_ms) * 1000000)
				AND status_code < 500) as satisfied_count,
			countIf(duration > (750000000 + toInt64(offset_ms) * 1000000)
				AND duration <= (1500000000 + toInt64(offset_ms) * 1000000)
				AND status_code < 500) as tolerating_count,
			countIf(duration > (1500000000 + toInt64(offset_ms) * 1000000)
				OR status_code >= 500) as bad_count,
			countIf(status_code >= 400 AND status_code < 500) as client_error_count
		FROM (
			SELECT e.endpoint, e.duration, e.status_code, e.recorded_at,
				   s.offset_ms as offset_ms
			FROM endpoints e
			LEFT JOIN (SELECT * FROM slow_endpoints FINAL) AS s
				ON e.endpoint = s.endpoint AND e.project_id = s.project_id
			WHERE e.project_id = ? AND e.recorded_at >= ? AND e.recorded_at <= ?
		)
		GROUP BY endpoint, offset_ms
	)
	ORDER BY impact DESC
	LIMIT ?`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.EndpointStats
	for rows.Next() {
		var s models.EndpointStats
		var p50, p95, p99, avg float64
		var offsetMs uint32
		var satisfiedCount, toleratingCount, badCount, clientErrorCount uint64
		if err := rows.Scan(&s.Endpoint, &s.Count, &p50, &p95, &p99, &avg, &s.LastSeen,
			&offsetMs, &satisfiedCount, &toleratingCount, &badCount, &clientErrorCount,
			&s.Impact); err != nil {
			return nil, err
		}
		s.P50Duration = time.Duration(p50)
		s.P95Duration = time.Duration(p95)
		s.P99Duration = time.Duration(p99)
		s.AvgDuration = time.Duration(avg)
		s.ImpactReason = computeImpactReason(s.Count, satisfiedCount, toleratingCount, badCount, clientErrorCount, p99, offsetMs)
		stats = append(stats, s)
	}

	return stats, nil
}

// GetEndpointStats returns aggregate statistics for a specific endpoint
func (e *endpointRepository) GetEndpointStats(ctx context.Context, projectId uuid.UUID, endpoint string, start, end time.Time) (*models.EndpointDetailStats, error) {
	// Calculate time range duration for throughput calculation
	durationMinutes := end.Sub(start).Minutes()
	if durationMinutes < 1 {
		durationMinutes = 1
	}

	query := `SELECT
		count() as count,
		if(count() > 0, avg(duration) / 1000000, 0) as avg_duration_ms,
		if(count() > 0, quantile(0.5)(duration) / 1000000, 0) as p50_duration_ms,
		if(count() > 0, quantile(0.95)(duration) / 1000000, 0) as p95_duration_ms,
		if(count() > 0, quantile(0.99)(duration) / 1000000, 0) as p99_duration_ms,
		if(count() > 0, countIf(status_code >= 500) * 100.0 / count(), 0) as error_rate,
		if(count() > 0,
			countIf(duration <= 500000000 AND status_code < 500) +
			(countIf(duration > 500000000 AND duration <= 2000000000 AND status_code < 500) * 0.5),
			0) as satisfied_tolerating
	FROM endpoints
	WHERE project_id = ? AND endpoint = ? AND recorded_at >= ? AND recorded_at <= ?`

	var stats models.EndpointDetailStats
	var count uint64
	var satisfiedTolerating float64

	err := (*chdb.Conn).QueryRow(ctx, query, projectId, endpoint, start, end).Scan(
		&count,
		&stats.AvgDuration,
		&stats.MedianDuration,
		&stats.P95Duration,
		&stats.P99Duration,
		&stats.ErrorRate,
		&satisfiedTolerating,
	)
	if err != nil {
		return nil, err
	}

	stats.Count = int64(count)
	// Calculate Apdex: (satisfied + tolerating*0.5) / total
	if count > 0 {
		stats.Apdex = satisfiedTolerating / float64(count)
	}
	// Calculate throughput (requests per minute)
	stats.Throughput = float64(count) / durationMinutes

	return &stats, nil
}

// GetEndpointStackedChart returns time-bucketed data for top 5 endpoints by metric + "Other"
func (e *endpointRepository) GetEndpointStackedChart(ctx context.Context, projectId uuid.UUID, start, end time.Time, intervalMinutes int, metricType string) (*models.EndpointStackedChartResponse, error) {
	// Step 1: Get top 5 endpoints ranked by selected metric
	var rankQuery string
	switch metricType {
	case "total_time":
		rankQuery = `SELECT endpoint, count() * quantile(0.5)(duration) / 1000000 as metric_value
			FROM endpoints
			WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
			GROUP BY endpoint
			ORDER BY metric_value DESC
			LIMIT 5`
	case "p95":
		rankQuery = `SELECT endpoint, quantile(0.95)(duration) / 1000000 as metric_value
			FROM endpoints
			WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
			GROUP BY endpoint
			ORDER BY metric_value DESC
			LIMIT 5`
	case "p99":
		rankQuery = `SELECT endpoint, quantile(0.99)(duration) / 1000000 as metric_value
			FROM endpoints
			WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
			GROUP BY endpoint
			ORDER BY metric_value DESC
			LIMIT 5`
	default: // p50
		rankQuery = `SELECT endpoint, quantile(0.5)(duration) / 1000000 as metric_value
			FROM endpoints
			WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
			GROUP BY endpoint
			ORDER BY metric_value DESC
			LIMIT 5`
	}

	rows, err := (*chdb.Conn).Query(ctx, rankQuery, projectId, start, end)
	if err != nil {
		return nil, err
	}

	var topEndpoints []string
	for rows.Next() {
		var endpoint string
		var metricValue float64
		if err := rows.Scan(&endpoint, &metricValue); err != nil {
			rows.Close()
			return nil, err
		}
		topEndpoints = append(topEndpoints, endpoint)
	}
	rows.Close()

	if len(topEndpoints) == 0 {
		return &models.EndpointStackedChartResponse{
			Endpoints: []string{},
			Series:    []models.EndpointTimeSeriesPoint{},
		}, nil
	}

	// Step 2: Build the time-bucketed query with top endpoints + "Other"
	var metricExpr string
	switch metricType {
	case "total_time":
		metricExpr = "count() * quantile(0.5)(duration) / 1000000"
	case "p95":
		metricExpr = "quantile(0.95)(duration) / 1000000"
	case "p99":
		metricExpr = "quantile(0.99)(duration) / 1000000"
	default: // p50
		metricExpr = "quantile(0.5)(duration) / 1000000"
	}

	// Build CASE expression for categorizing endpoints
	caseExpr := "multiIf("
	for i, ep := range topEndpoints {
		caseExpr += "endpoint = ?, '" + ep + "', "
		_ = i // use index in iteration
	}
	caseExpr += "'Other')"

	timeSeriesQuery := `SELECT
		toStartOfInterval(recorded_at, INTERVAL ? MINUTE) as bucket,
		` + caseExpr + ` as endpoint_category,
		` + metricExpr + ` as metric_value
	FROM endpoints
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY bucket, endpoint_category
	ORDER BY bucket ASC, endpoint_category ASC`

	// Build args: interval, top endpoints for CASE, then project_id, start, end
	args := make([]interface{}, 0, len(topEndpoints)+4)
	args = append(args, intervalMinutes)
	for _, ep := range topEndpoints {
		args = append(args, ep)
	}
	args = append(args, projectId, start, end)

	rows, err = (*chdb.Conn).Query(ctx, timeSeriesQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []models.EndpointTimeSeriesPoint
	for rows.Next() {
		var p models.EndpointTimeSeriesPoint
		if err := rows.Scan(&p.Timestamp, &p.Endpoint, &p.Value); err != nil {
			return nil, err
		}
		series = append(series, p)
	}

	// Build final endpoint list (top 5 + Other if there are other endpoints)
	endpointSet := make(map[string]bool)
	for _, p := range series {
		endpointSet[p.Endpoint] = true
	}

	finalEndpoints := make([]string, 0, len(topEndpoints)+1)
	finalEndpoints = append(finalEndpoints, topEndpoints...)
	if endpointSet["Other"] {
		finalEndpoints = append(finalEndpoints, "Other")
	}

	return &models.EndpointStackedChartResponse{
		Endpoints: finalEndpoints,
		Series:    series,
	}, nil
}

func (e *endpointRepository) GetSlowEndpoint(ctx context.Context, projectId uuid.UUID, endpoint string) (uint32, string, error) {
	var offsetMs uint32
	var reason string
	err := (*chdb.Conn).QueryRow(ctx, "SELECT offset_ms, reason FROM slow_endpoints FINAL WHERE project_id = ? AND endpoint = ?", projectId, endpoint).Scan(&offsetMs, &reason)
	return offsetMs, reason, err
}

func (e *endpointRepository) UpsertSlowEndpoint(ctx context.Context, projectId uuid.UUID, endpoint string, offsetMs uint32, reason string) error {
	err := (*chdb.Conn).Exec(ctx, "ALTER TABLE slow_endpoints DELETE WHERE project_id = ? AND endpoint = ?", projectId, endpoint)
	if err != nil {
		return err
	}
	batch, err := (*chdb.Conn).PrepareBatch(ctx, "INSERT INTO slow_endpoints (project_id, endpoint, offset_ms, reason)")
	if err != nil {
		return err
	}
	if err := batch.Append(projectId, endpoint, offsetMs, reason); err != nil {
		return err
	}
	return batch.Send()
}

var EndpointRepository = endpointRepository{}
