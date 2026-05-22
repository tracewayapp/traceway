//go:build !pgch

package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type span struct {
	Id           uuid.UUID  `lit:"id"`
	TraceId      uuid.UUID  `lit:"trace_id"`
	ProjectId    uuid.UUID  `lit:"project_id"`
	Name         string     `lit:"name"`
	StartTime    SQLiteTime `lit:"start_time"`
	Duration     int64      `lit:"duration"`
	RecordedAt   SQLiteTime `lit:"recorded_at"`
	ParentSpanId *uuid.UUID `lit:"parent_span_id"`
	EntityId     *uuid.UUID `lit:"entity_id"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[span](driver)
	})
}

func spanToRow(s models.Span) span {
	return span{
		Id:           s.Id,
		TraceId:      s.TraceId,
		ProjectId:    s.ProjectId,
		Name:         s.Name,
		StartTime:    NewSQLiteTime(s.StartTime),
		Duration:     int64(s.Duration),
		RecordedAt:   NewSQLiteTime(s.RecordedAt),
		ParentSpanId: s.ParentSpanId,
		EntityId:     s.EntityId,
	}
}

func (r *span) toModel() models.Span {
	return models.Span{
		Id:           r.Id,
		TraceId:      r.TraceId,
		ProjectId:    r.ProjectId,
		Name:         r.Name,
		StartTime:    r.StartTime.Time,
		Duration:     time.Duration(r.Duration),
		RecordedAt:   r.RecordedAt.Time,
		ParentSpanId: r.ParentSpanId,
		EntityId:     r.EntityId,
	}
}

type spanRepository struct{}

func (r *spanRepository) InsertAsync(ctx context.Context, spans []models.Span) error {
	if len(spans) == 0 {
		return nil
	}

	for _, s := range spans {
		row := spanToRow(s)
		if err := lit.InsertExistingUuid(db.TelemetryDB, &row); err != nil {
			return err
		}
	}

	return nil
}

func (r *spanRepository) FindByTraceId(ctx context.Context, projectId, traceId uuid.UUID) ([]models.Span, error) {
	rows, err := lit.SelectNamed[span](db.TelemetryDB,
		`SELECT id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, entity_id
		FROM spans
		WHERE project_id = :project_id AND trace_id = :trace_id
		ORDER BY start_time ASC`,
		lit.P{"project_id": projectId, "trace_id": traceId})
	if err != nil {
		return nil, err
	}

	spans := make([]models.Span, 0, len(rows))
	for _, row := range rows {
		spans = append(spans, row.toModel())
	}
	return spans, nil
}

func (r *spanRepository) FindByEntityId(ctx context.Context, projectId, entityId uuid.UUID) ([]models.Span, error) {
	rows, err := lit.SelectNamed[span](db.TelemetryDB,
		`SELECT id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, entity_id
		FROM spans
		WHERE project_id = :project_id AND entity_id = :entity_id
		ORDER BY start_time ASC`,
		lit.P{"project_id": projectId, "entity_id": entityId})
	if err != nil {
		return nil, err
	}

	spans := make([]models.Span, 0, len(rows))
	for _, row := range rows {
		spans = append(spans, row.toModel())
	}
	return spans, nil
}

func (r *spanRepository) FindById(ctx context.Context, projectId, spanId uuid.UUID) (*models.Span, error) {
	row, err := lit.SelectSingleNamed[span](db.TelemetryDB,
		`SELECT id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, entity_id
		FROM spans
		WHERE project_id = :project_id AND id = :id
		LIMIT 1`,
		lit.P{"project_id": projectId, "id": spanId})
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	s := row.toModel()
	return &s, nil
}

var SpanRepository = spanRepository{}
