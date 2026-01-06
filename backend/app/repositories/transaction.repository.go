package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
	"time"
)

type transactionRepository struct{}

func (e *transactionRepository) InsertAsync(ctx context.Context, lines []models.Transaction) error {
	batch, err := (*chdb.Conn).PrepareBatch(ctx, "INSERT INTO transactions (id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip)")
	if err != nil {
		return err
	}
	for _, t := range lines {
		if err := batch.Append(t.Id, t.ProjectId, t.Endpoint, t.Duration, t.RecordedAt, t.StatusCode, t.BodySize, t.ClientIP); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (e *transactionRepository) CountBetween(ctx context.Context, projectId string, start, end time.Time) (int64, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT count() FROM transactions WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, start, end).Scan(&count)
	return int64(count), err
}

func (e *transactionRepository) FindAll(ctx context.Context, projectId string, fromDate, toDate time.Time, page, pageSize int, orderBy string) ([]models.Transaction, int64, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT count() FROM transactions WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, fromDate, toDate).Scan(&count)
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

	query := "SELECT id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip FROM transactions WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY " + orderBy + " DESC LIMIT ? OFFSET ?"
	rows, err := (*chdb.Conn).Query(ctx, query, projectId, fromDate, toDate, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(&t.Id, &t.ProjectId, &t.Endpoint, &t.Duration, &t.RecordedAt, &t.StatusCode, &t.BodySize, &t.ClientIP); err != nil {
			return nil, 0, err
		}
		transactions = append(transactions, t)
	}

	return transactions, int64(count), nil
}

func (e *transactionRepository) FindGroupedByEndpoint(ctx context.Context, projectId string, fromDate, toDate time.Time, page, pageSize int, orderBy string) ([]models.EndpointStats, int64, error) {
	// Count unique endpoints
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT uniq(endpoint) FROM transactions WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, fromDate, toDate).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize

	allowedOrderBy := map[string]bool{
		"count":        true,
		"p50_duration": true,
		"p95_duration": true,
		"avg_duration": true,
		"last_seen":    true,
	}

	if !allowedOrderBy[orderBy] {
		orderBy = "count"
	}

	query := `SELECT
		endpoint,
		count() as count,
		quantile(0.5)(duration) as p50_duration,
		quantile(0.95)(duration) as p95_duration,
		avg(duration) as avg_duration,
		max(recorded_at) as last_seen
	FROM transactions
	WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY endpoint
	ORDER BY ` + orderBy + ` DESC
	LIMIT ? OFFSET ?`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, fromDate, toDate, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var stats []models.EndpointStats
	for rows.Next() {
		var s models.EndpointStats
		var p50, p95, avg float64
		if err := rows.Scan(&s.Endpoint, &s.Count, &p50, &p95, &avg, &s.LastSeen); err != nil {
			return nil, 0, err
		}
		s.P50Duration = time.Duration(p50)
		s.P95Duration = time.Duration(p95)
		s.AvgDuration = time.Duration(avg)
		stats = append(stats, s)
	}

	return stats, int64(count), nil
}

func (e *transactionRepository) FindByEndpoint(ctx context.Context, projectId string, endpoint string, fromDate, toDate time.Time, page, pageSize int, orderBy string) ([]models.Transaction, int64, error) {
	var count uint64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT count() FROM transactions WHERE project_id = ? AND endpoint = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, endpoint, fromDate, toDate).Scan(&count)
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

	query := "SELECT id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip FROM transactions WHERE project_id = ? AND endpoint = ? AND recorded_at >= ? AND recorded_at <= ? ORDER BY " + orderBy + " DESC LIMIT ? OFFSET ?"
	rows, err := (*chdb.Conn).Query(ctx, query, projectId, endpoint, fromDate, toDate, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(&t.Id, &t.ProjectId, &t.Endpoint, &t.Duration, &t.RecordedAt, &t.StatusCode, &t.BodySize, &t.ClientIP); err != nil {
			return nil, 0, err
		}
		transactions = append(transactions, t)
	}

	return transactions, int64(count), nil
}

// CountByHour returns transaction counts grouped by hour
func (e *transactionRepository) CountByHour(ctx context.Context, projectId string, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfHour(recorded_at) as hour,
		count() as count
	FROM transactions
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
func (e *transactionRepository) AvgDurationByHour(ctx context.Context, projectId string, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfHour(recorded_at) as hour,
		avg(duration) / 1000000 as avg_duration_ms
	FROM transactions
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
func (e *transactionRepository) ErrorRateByHour(ctx context.Context, projectId string, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfHour(recorded_at) as hour,
		countIf(status_code >= 400) * 100.0 / count() as error_rate
	FROM transactions
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

var TransactionRepository = transactionRepository{}
