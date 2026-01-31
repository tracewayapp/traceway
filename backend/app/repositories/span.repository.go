package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

type spanRepository struct{}

func (r *spanRepository) InsertAsync(ctx context.Context, spans []models.Span) error {
	if len(spans) == 0 {
		return nil
	}

	batch, err := (*chdb.Conn).PrepareBatch(clickhouse.Context(context.Background(), clickhouse.WithAsync(false)),
		"INSERT INTO spans (id, trace_id, project_id, name, start_time, duration, recorded_at)")
	if err != nil {
		return err
	}

	for _, s := range spans {
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

func (r *spanRepository) FindByTraceId(ctx context.Context, projectId, traceId uuid.UUID) ([]models.Span, error) {
	query := `SELECT
		id, trace_id, project_id, name, start_time, duration, recorded_at
	FROM spans
	WHERE project_id = ? AND trace_id = ?
	ORDER BY start_time ASC`

	rows, err := (*chdb.Conn).Query(ctx, query, projectId, traceId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spans []models.Span
	for rows.Next() {
		var s models.Span
		if err := rows.Scan(
			&s.Id, &s.TraceId, &s.ProjectId,
			&s.Name, &s.StartTime, &s.Duration, &s.RecordedAt,
		); err != nil {
			return nil, err
		}
		spans = append(spans, s)
	}

	return spans, nil
}

var SpanRepository = spanRepository{}
