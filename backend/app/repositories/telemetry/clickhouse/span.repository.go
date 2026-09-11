//go:build telemetry_ch

package clickhouse

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/tracewayapp/traceway/backend/app/chdb"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"

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
	rows, err := chdb.Conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
