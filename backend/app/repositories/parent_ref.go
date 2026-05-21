package repositories

import (
	"context"

	"github.com/google/uuid"
)

type AiTraceRef struct {
	Id        uuid.UUID
	TraceName string
}

type ParentRef struct {
	Kind    string
	Id      uuid.UUID
	Name    string
	TraceId uuid.UUID
}

const findParentRefMaxDepth = 10

// FindParentRef walks up the parent_span_id chain starting at parentSpanId
// looking for the nearest row in endpoints / tasks / ai_traces. Generic spans
// in the chain are followed through to their own parent. Returns nil if the
// chain reaches a span with no parent or the depth cap fires before finding a
// special-table ancestor.
func FindParentRef(ctx context.Context, projectId, parentSpanId uuid.UUID) (*ParentRef, error) {
	current := parentSpanId
	for i := 0; i < findParentRefMaxDepth; i++ {
		if current == uuid.Nil {
			return nil, nil
		}

		ep, err := EndpointRepository.FindById(ctx, projectId, current)
		if err != nil {
			return nil, err
		}
		if ep != nil {
			return &ParentRef{Kind: "endpoint", Id: ep.Id, Name: ep.Endpoint, TraceId: ep.TraceId}, nil
		}

		tk, err := TaskRepository.FindById(ctx, projectId, current)
		if err != nil {
			return nil, err
		}
		if tk != nil {
			return &ParentRef{Kind: "task", Id: tk.Id, Name: tk.TaskName, TraceId: tk.TraceId}, nil
		}

		ai, err := AiTraceRepository.FindById(ctx, projectId, current)
		if err != nil {
			return nil, err
		}
		if ai != nil {
			return &ParentRef{Kind: "ai_trace", Id: ai.Id, Name: ai.TraceName, TraceId: ai.TraceId}, nil
		}

		sp, err := SpanRepository.FindById(ctx, projectId, current)
		if err != nil {
			return nil, err
		}
		if sp == nil || sp.ParentSpanId == nil {
			return nil, nil
		}
		current = *sp.ParentSpanId
	}
	return nil, nil
}
