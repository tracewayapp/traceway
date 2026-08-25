//go:build telemetry_firebolt

package firebolt

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/sqlitetypes"
)

type span struct {
	Id           uuid.UUID                 `lit:"id"`
	TraceId      uuid.UUID                 `lit:"trace_id"`
	ProjectId    uuid.UUID                 `lit:"project_id"`
	Name         string                    `lit:"name"`
	StartTime    sqlitetypes.SQLiteTime    `lit:"start_time"`
	Duration     int64                     `lit:"duration"`
	RecordedAt   sqlitetypes.SQLiteTime    `lit:"recorded_at"`
	ParentSpanId *uuid.UUID                `lit:"parent_span_id"`
	Attributes   sqlitetypes.SQLiteJSONMap `lit:"attributes"`
}

func init() {
	registerModels(func(driver lit.Driver) {
		lit.RegisterModel[span](driver)
	})
}

func (r *span) toModel() models.Span {
	s := models.Span{
		Id:           r.Id,
		TraceId:      r.TraceId,
		ProjectId:    r.ProjectId,
		Name:         r.Name,
		StartTime:    r.StartTime.Time,
		Duration:     time.Duration(r.Duration),
		RecordedAt:   r.RecordedAt.Time,
		ParentSpanId: r.ParentSpanId,
	}
	if r.Attributes != nil {
		s.Attributes = map[string]string(r.Attributes)
	}
	return s
}

type spanRepository struct{}

var spanColumns = []string{"id", "trace_id", "project_id", "name", "start_time", "duration", "recorded_at", "parent_span_id", "attributes"}

func (r *spanRepository) InsertAsync(ctx context.Context, spans []models.Span) error {
	if len(spans) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(spans))
	for _, s := range spans {
		attributesJSON, err := attrJSON(s.Attributes)
		if err != nil {
			return err
		}

		var parentSpanId *string
		if s.ParentSpanId != nil {
			v := s.ParentSpanId.String()
			parentSpanId = &v
		}

		rows = append(rows, []any{
			s.Id.String(),
			s.TraceId.String(),
			s.ProjectId.String(),
			s.Name,
			s.StartTime.UTC(),
			int64(s.Duration),
			s.RecordedAt.UTC(),
			nullableString(parentSpanId),
			attributesJSON,
		})
	}
	return insertRows(ctx, "spans", spanColumns, rows)
}

func (r *spanRepository) FindByTraceId(ctx context.Context, projectId, traceId uuid.UUID, recordedAt *time.Time) ([]models.Span, error) {
	query := `SELECT id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, attributes
		FROM spans
		WHERE project_id = :project_id AND trace_id = :trace_id`
	params := lit.P{"project_id": projectId, "trace_id": traceId}
	if recordedAt != nil {
		from, to := shared.TraceWindowBounds(*recordedAt)
		query += ` AND recorded_at >= :from AND recorded_at <= :to`
		params["from"] = from.UTC()
		params["to"] = to.UTC()
	}
	query += ` ORDER BY start_time ASC`

	rows, err := lit.SelectNamed[span](db.TelemetryDB, query, params)
	if err != nil {
		return nil, err
	}

	spans := make([]models.Span, 0, len(rows))
	for _, row := range rows {
		spans = append(spans, row.toModel())
	}
	return spans, nil
}

var SpanRepository = &spanRepository{}
