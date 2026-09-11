//go:build telemetry_duckdb

package duckdb

import (
	"context"
	"slices"
	"time"

	"github.com/duckdb/duckdb-go/v2"
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

	return withAppender(ctx, "spans", func(appender *duckdb.Appender) {

		for _, s := range spans {
			attributesJSON, err := attrJSON(s.Attributes)
			if err != nil {
				captureDroppedRow("spans", err)
				continue
			}

			var parentSpanId *string
			if s.ParentSpanId != nil {
				v := s.ParentSpanId.String()
				parentSpanId = &v
			}

			if err := appender.AppendRow(
				s.Id.String(),
				s.TraceId.String(),
				s.ProjectId.String(),
				s.Name,
				s.StartTime.UTC(),
				int64(s.Duration),
				s.RecordedAt.UTC(),
				nullableString(parentSpanId),
				attributesJSON,
			); err != nil {
				captureDroppedRow("spans", err)
			}
		}

	})
}

func (r *spanRepository) FindByTraceId(ctx context.Context, projectId, traceId uuid.UUID, recordedAt *time.Time) ([]models.Span, error) {
	query, args := shared.SpanLookupQuery([]shared.SpanLookup{{ProjectId: projectId, TraceId: traceId, RecordedAt: recordedAt}},
		func(t time.Time) any { return t.UTC() })
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
		query, args := shared.SpanLookupQuery(batch, func(t time.Time) any { return t.UTC() })
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
