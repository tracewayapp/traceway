package telemetry

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

type spanGraphRepository struct{}

// InsertAsync stores spans that did not arrive as OTLP, the ones of the native protocol, next to the OTLP ones.
func (r *spanGraphRepository) InsertAsync(ctx context.Context, spans []models.Span) error {
	stored := make([]models.OtelSpan, len(spans))
	for i, span := range spans {
		stored[i] = models.OtelSpan{Span: span}
	}
	_, err := OtelSpanRepository.InsertAsync(ctx, stored)
	return err
}

// FindGraph returns the spans under one span of a trace: the waterfall of an endpoint, a task or an AI trace.
func (r *spanGraphRepository) FindGraph(ctx context.Context, lookup shared.SpanLookup) (*shared.SpanGraph, error) {
	graphs, err := r.FindGraphs(ctx, []shared.SpanLookup{lookup})
	if err != nil {
		return nil, err
	}
	if graph, ok := graphs[lookup.Owner()]; ok {
		return graph, nil
	}
	return &shared.SpanGraph{Spans: []models.Span{}, Status: models.SpanGraphStatus{State: models.SpanGraphComplete}}, nil
}

func (r *spanGraphRepository) FindGraphs(ctx context.Context, lookups []shared.SpanLookup) (map[shared.SpanOwner]*shared.SpanGraph, error) {
	return shared.FindOtelGraphs(ctx, lookups, OtelSpanRepository)
}

func traceLookups(projectIds []uuid.UUID, traceId string, at time.Time) []shared.SpanLookup {
	lookups := make([]shared.SpanLookup, len(projectIds))
	for i, projectId := range projectIds {
		lookups[i] = shared.SpanLookup{ProjectId: projectId, TraceId: traceId, RecordedAt: &at}
	}
	return lookups
}

// FindTrace returns the stored spans of one trace from every project listed, for the span explorer. The caller decides
// which projects the user may read. The read is anchored on at, like every other.
func (r *spanGraphRepository) FindTrace(ctx context.Context, projectIds []uuid.UUID, traceId string, at time.Time) (*shared.SpanGraph, error) {
	return shared.FindOtelTrace(ctx, traceLookups(projectIds, traceId, at), OtelSpanRepository)
}

// FindTraceParents maps each span of a trace to its parent span, read from every project listed.
func (r *spanGraphRepository) FindTraceParents(ctx context.Context, projectIds []uuid.UUID, traceId string, at time.Time) (map[string]string, error) {
	return shared.FindOtelTraceParents(ctx, traceLookups(projectIds, traceId, at), OtelSpanRepository)
}

// FindSpanAttributes loads one span's attributes for the explorer's popover. nil means the span is not stored near at.
func (r *spanGraphRepository) FindSpanAttributes(ctx context.Context, projectId uuid.UUID, traceId, spanId string, at time.Time) (*shared.OtelSpanAttributes, error) {
	return shared.FindOtelSpanAttributes(ctx, shared.SpanLookup{ProjectId: projectId, TraceId: traceId, RecordedAt: &at}, spanId, OtelSpanRepository)
}

var SpanRepository = &spanGraphRepository{}
