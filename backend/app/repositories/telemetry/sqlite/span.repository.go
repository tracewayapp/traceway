//go:build !telemetry_ch && !telemetry_duckdb

package sqlite

import (
	"context"
	"slices"
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
		StartTime:    sqlitetypes.NewSQLiteTime(s.StartTime),
		Duration:     int64(s.Duration),
		RecordedAt:   sqlitetypes.NewSQLiteTime(s.RecordedAt),
		ParentSpanId: s.ParentSpanId,
		Attributes:   sqlitetypes.NewSQLiteJSONMap(s.Attributes),
	}
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

func (r *spanRepository) InsertAsync(ctx context.Context, spans []models.Span) error {
	if len(spans) == 0 {
		return nil
	}

	tx, err := db.TelemetryDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, s := range spans {
		row := spanToRow(s)
		if err := lit.InsertExistingUuid(tx, &row); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *spanRepository) FindByTraceId(ctx context.Context, projectId, traceId uuid.UUID, recordedAt *time.Time) ([]models.Span, error) {
	query, args := shared.SpanLookupQuery([]shared.SpanLookup{{ProjectId: projectId, TraceId: traceId, RecordedAt: recordedAt}},
		func(t time.Time) any { return sqlitetypes.NewSQLiteTime(t) })
	return r.querySpans(ctx, query, args)
}

func (r *spanRepository) FindByTraces(ctx context.Context, lookups []shared.SpanLookup) ([]models.Span, error) {
	spans := make([]models.Span, 0)
	type spanKey struct {
		projectId, traceId, id uuid.UUID
		recordedAt             time.Time
	}
	seen := make(map[spanKey]bool)
	for batch := range slices.Chunk(lookups, shared.SpanLookupBatchSize) {
		query, args := shared.SpanLookupQuery(batch, func(t time.Time) any { return sqlitetypes.NewSQLiteTime(t) })
		found, err := r.querySpans(ctx, query, args)
		if err != nil {
			return nil, err
		}
		for _, span := range found {
			key := spanKey{span.ProjectId, span.TraceId, span.Id, span.RecordedAt}
			if !seen[key] {
				seen[key] = true
				spans = append(spans, span)
			}
		}
	}
	slices.SortStableFunc(spans, func(a, b models.Span) int { return a.StartTime.Compare(b.StartTime) })
	return spans, nil
}

func (r *spanRepository) querySpans(ctx context.Context, query string, args []any) ([]models.Span, error) {
	rows, err := db.TelemetryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	spans := make([]models.Span, 0)
	for rows.Next() {
		var row span
		if err := rows.Scan(&row.Id, &row.TraceId, &row.ProjectId, &row.Name, &row.StartTime,
			&row.Duration, &row.RecordedAt, &row.ParentSpanId, &row.Attributes); err != nil {
			return nil, err
		}
		spans = append(spans, row.toModel())
	}
	return spans, rows.Err()
}

var SpanRepository = &spanRepository{}
