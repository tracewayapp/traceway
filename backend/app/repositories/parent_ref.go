package repositories

import (
	"context"
	"sort"
	"time"

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

// ChildEntityRef describes a child entity (endpoint/task/ai_trace) whose
// parent_span_id falls inside another entity's owned-span subtree. Used by the
// detail controllers to surface "things that happened under this row" without
// re-introducing the cross-subtree leak that Phase 2 fixed.
type ChildEntityRef struct {
	Kind         string
	Id           uuid.UUID
	Name         string
	ParentSpanId uuid.UUID
	TraceId      uuid.UUID
	RecordedAt   time.Time
	Duration     time.Duration
}

// FindChildEntitiesBySpanIds returns every endpoint/task/ai_trace row whose
// parent_span_id is in the given set, sorted by RecordedAt ascending so the
// frontend can render them chronologically in the waterfall. Empty input
// short-circuits to an empty result (no query).
func FindChildEntitiesBySpanIds(ctx context.Context, projectId uuid.UUID, spanIds []uuid.UUID) ([]ChildEntityRef, error) {
	if len(spanIds) == 0 {
		return nil, nil
	}

	endpoints, err := EndpointRepository.FindByParentSpanIds(ctx, projectId, spanIds)
	if err != nil {
		return nil, err
	}
	tasks, err := TaskRepository.FindByParentSpanIds(ctx, projectId, spanIds)
	if err != nil {
		return nil, err
	}
	ais, err := AiTraceRepository.FindByParentSpanIds(ctx, projectId, spanIds)
	if err != nil {
		return nil, err
	}

	out := make([]ChildEntityRef, 0, len(endpoints)+len(tasks)+len(ais))
	out = append(out, endpoints...)
	out = append(out, tasks...)
	out = append(out, ais...)

	sort.Slice(out, func(i, j int) bool { return out[i].RecordedAt.Before(out[j].RecordedAt) })
	return out, nil
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
