package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
)

type metricRecordRepository struct{}

func (e *metricRecordRepository) InsertAsync(ctx context.Context, lines []models.MetricRecord) error {
	batch, err := (*chdb.Conn).PrepareBatch(ctx, "INSERT INTO metric_records (name, value, recorded_at)")
	if err != nil {
		return err
	}
	for _, e := range lines {
		if err := batch.Append(e.Name, e.Value, e.RecordedAt); err != nil {
			return err
		}
	}
	return batch.Send()
}

var MetricRecordRepository = metricRecordRepository{}
