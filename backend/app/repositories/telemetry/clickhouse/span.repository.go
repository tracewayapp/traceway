//go:build telemetry_ch

package clickhouse

import (
	"context"
	"encoding/json"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"time"

	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
)

type spanRepository struct{}

func (r *spanRepository) InsertAsync(ctx context.Context, spans []models.Span) error {
	if len(spans) == 0 {
		return nil
	}

	return chdb.SendBatch("INSERT INTO spans (id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, attributes)", func(batch driver.Batch) error {
		for _, s := range spans {
			attributesJSON := "{}"
			if len(s.Attributes) != 0 {
				if attributesBytes, err := json.Marshal(s.Attributes); err == nil {
					attributesJSON = string(attributesBytes)
				}
			}
			if err := batch.Append(
				s.Id,
				s.TraceId,
				s.ProjectId,
				s.Name,
				s.StartTime,
				int64(s.Duration),
				s.RecordedAt,
				s.ParentSpanId,
				attributesJSON,
			); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *spanRepository) FindByTraceId(ctx context.Context, projectId, traceId uuid.UUID, recordedAt *time.Time) ([]models.Span, error) {
	query := `SELECT
		id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, attributes
	FROM spans
	WHERE project_id = ? AND trace_id = ?`
	args := []any{projectId, traceId}
	if recordedAt != nil {
		from, to := shared.TraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= ? AND recorded_at <= ?`
		args = append(args, from, to)
	}
	query += ` ORDER BY start_time ASC`

	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSpans(rows)
}

// FindByTraceIds loads the spans of every referenced trace in one query, for
// views that show several entities side by side. The lookup is bounded to the
// trace window around the earliest and latest reference and follows the
// (project_id, trace_id) ordering key.
func (r *spanRepository) FindByTraceIds(ctx context.Context, refs []models.TraceRef) ([]models.Span, error) {
	if len(refs) == 0 {
		return []models.Span{}, nil
	}
	projectIds, traceIds := shared.TraceRefIds(refs)
	from, to := shared.TraceRefsWindowBounds(refs)
	query := `SELECT
		id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, attributes
	FROM spans
	WHERE project_id IN (?) AND trace_id IN (?) AND recorded_at >= ? AND recorded_at <= ?
	ORDER BY start_time ASC`

	rows, err := chdb.Conn.Query(ctx, query, projectIds, traceIds, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	spans, err := scanSpans(rows)
	if err != nil {
		return nil, err
	}
	return shared.FilterSpansByTraceRefs(spans, refs), nil
}

func scanSpans(rows driver.Rows) ([]models.Span, error) {
	var spans []models.Span
	for rows.Next() {
		var s models.Span
		var attributesJSON string
		if err := rows.Scan(
			&s.Id, &s.TraceId, &s.ProjectId,
			&s.Name, &s.StartTime, &s.Duration, &s.RecordedAt, &s.ParentSpanId,
			&attributesJSON,
		); err != nil {
			return nil, err
		}
		if attributesJSON != "" && attributesJSON != "{}" {
			if err := json.Unmarshal([]byte(attributesJSON), &s.Attributes); err != nil {
				s.Attributes = nil
			}
		}
		spans = append(spans, s)
	}
	return spans, rows.Err()
}

var SpanRepository = &spanRepository{}
