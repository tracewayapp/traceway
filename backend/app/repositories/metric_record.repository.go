package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
	"time"
)

type metricRecordRepository struct{}

func (e *metricRecordRepository) InsertAsync(ctx context.Context, lines []models.MetricRecord) error {
	batch, err := (*chdb.Conn).PrepareBatch(ctx, "INSERT INTO metric_records (project_id, name, value, recorded_at)")
	if err != nil {
		return err
	}
	for _, m := range lines {
		if err := batch.Append(m.ProjectId, m.Name, m.Value, m.RecordedAt); err != nil {
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

var MetricRecordRepository = metricRecordRepository{}
