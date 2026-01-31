package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

type segmentRepository struct{}

func (r *segmentRepository) InsertAsync(ctx context.Context, segments []models.Segment) error {
	if len(segments) == 0 {
		return nil
	}

	batch, err := (*chdb.Conn).PrepareBatch(clickhouse.Context(context.Background(), clickhouse.WithAsync(false)),
		"INSERT INTO segments (id, trace_id, project_id, name, start_time, duration, recorded_at)")
	if err != nil {
		return err
	}

	for _, s := range segments {
		if err := batch.Append(
			s.Id,
			s.TraceId,
			s.ProjectId,
			s.Name,
			s.StartTime,
			s.Duration,
			s.RecordedAt,
		); err != nil {
			return err
		}
	}

	return batch.Send()
}

func (r *segmentRepository) FindByTraceId(ctx context.Context, projectId, traceId uuid.UUID) ([]models.Segment, error) {
	query := `SELECT
		id, trace_id, project_id, name, start_time, duration, recorded_at
	FROM segments
	WHERE project_id = ? AND trace_id = ?
	ORDER BY start_time ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, traceId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segments []models.Segment
	for rows.Next() {
		var s models.Segment
		if err := rows.Scan(
			&s.Id, &s.TraceId, &s.ProjectId,
			&s.Name, &s.StartTime, &s.Duration, &s.RecordedAt,
		); err != nil {
			return nil, err
		}
		segments = append(segments, s)
	}

	return segments, nil
}

var SegmentRepository = segmentRepository{}
