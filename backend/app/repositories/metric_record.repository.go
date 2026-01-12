package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
	"time"
)

type metricRecordRepository struct{}

func (e *metricRecordRepository) InsertAsync(ctx context.Context, lines []models.MetricRecord) error {
	batch, err := (*chdb.Conn).PrepareBatch(ctx, "INSERT INTO metric_records (project_id, name, value, recorded_at, server_name)")
	if err != nil {
		return err
	}
	for _, m := range lines {
		if err := batch.Append(m.ProjectId, m.Name, m.Value, m.RecordedAt, m.ServerName); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (e *metricRecordRepository) GetAverageBetween(ctx context.Context, projectId string, name string, start, end time.Time) (float64, error) {
	var avg float64
	err := (*chdb.Conn).QueryRow(ctx, "SELECT coalesce(avg(value), 0) FROM metric_records WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?", projectId, name, start, end).Scan(&avg)
	return avg, err
}

// GetAverageByHour returns metric averages grouped by hour
func (e *metricRecordRepository) GetAverageByHour(ctx context.Context, projectId string, name string, start, end time.Time) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfHour(recorded_at) as hour,
		avg(value) as avg_value
	FROM metric_records
	WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY hour
	ORDER BY hour ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, name, start, end)
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

// GetAverageByInterval returns metric averages grouped by configurable interval in minutes
func (e *metricRecordRepository) GetAverageByInterval(ctx context.Context, projectId string, name string, start, end time.Time, intervalMinutes int) ([]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfInterval(recorded_at, INTERVAL ? MINUTE) as bucket,
		avg(value) as avg_value
	FROM metric_records
	WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?
	GROUP BY bucket
	ORDER BY bucket ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, intervalMinutes, projectId, name, start, end)
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

// GetDistinctServers returns all unique server names with data in the time range
func (e *metricRecordRepository) GetDistinctServers(ctx context.Context, projectId string, start, end time.Time) ([]string, error) {
	query := `SELECT DISTINCT server_name FROM metric_records
              WHERE project_id = ? AND recorded_at >= ? AND recorded_at <= ?
              AND server_name != ''
              ORDER BY server_name ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, nil
}

// GetAverageByIntervalPerServer returns metric averages grouped by interval and server
func (e *metricRecordRepository) GetAverageByIntervalPerServer(ctx context.Context, projectId string, name string, start, end time.Time, intervalMinutes int, servers []string) (map[string][]models.TimeSeriesPoint, error) {
	query := `SELECT
		toStartOfInterval(recorded_at, INTERVAL ? MINUTE) as bucket,
		server_name,
		avg(value) as avg_value
	FROM metric_records
	WHERE project_id = ? AND name = ? AND recorded_at >= ? AND recorded_at <= ?`

	args := []interface{}{intervalMinutes, projectId, name, start, end}

	if len(servers) > 0 {
		query += " AND server_name IN (?)"
		args = append(args, servers)
	} else {
		query += " AND server_name != ''"
	}

	query += " GROUP BY bucket, server_name ORDER BY bucket ASC, server_name ASC"

	rows, err := (*chdb.Conn).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]models.TimeSeriesPoint)
	for rows.Next() {
		var bucket time.Time
		var serverName string
		var value float64
		if err := rows.Scan(&bucket, &serverName, &value); err != nil {
			return nil, err
		}
		result[serverName] = append(result[serverName], models.TimeSeriesPoint{
			Timestamp: bucket,
			Value:     value,
		})
	}
	return result, nil
}

var MetricRecordRepository = metricRecordRepository{}
