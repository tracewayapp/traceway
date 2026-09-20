package shared

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type graphReader struct {
	topology   func(context.Context, []SpanLookup, uint64) ([]models.OtelSpan, error)
	attributes func(context.Context, SpanLookup, []string, OtelAttributeLimits) (map[string]OtelSpanAttributes, error)
}

func (r graphReader) FindTraceTopology(ctx context.Context, lookups []SpanLookup, from uint64) ([]models.OtelSpan, error) {
	return r.topology(ctx, lookups, from)
}
func (r graphReader) FindSpanAttributes(ctx context.Context, lookup SpanLookup, ids []string, limits OtelAttributeLimits) (map[string]OtelSpanAttributes, error) {
	if r.attributes == nil {
		return nil, nil
	}
	return r.attributes(ctx, lookup, ids, limits)
}
func (r graphReader) IsReadLimitError(error) bool { return false }

func (r graphReader) FindTraceOutline(ctx context.Context, lookups []SpanLookup) ([]models.OtelSpan, error) {
	return r.topology(ctx, lookups, 0)
}

func TestOtelGraphsBatchQueries(t *testing.T) {
	project := uuid.New()
	lookups := make([]SpanLookup, SpanLookupBatchSize+1)
	for i := range lookups {
		lookups[i] = SpanLookup{ProjectId: project, TraceId: NormalizeTraceId(uuid.NewString()), SpanId: "0102030405060708"}
	}
	calls := 0
	reader := graphReader{topology: func(ctx context.Context, batch []SpanLookup, from uint64) ([]models.OtelSpan, error) {
		calls++
		if len(batch) > SpanLookupBatchSize || from != 0 {
			t.Fatalf("unexpected batch: %d, from %d", len(batch), from)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("graph query has no deadline")
		}
		roots := make([]models.OtelSpan, len(batch))
		for i, lookup := range batch {
			roots[i] = models.OtelSpan{Span: models.Span{ProjectId: project, TraceId: lookup.TraceId, SpanId: lookup.SpanId}}
		}
		return roots, nil
	}}
	found, err := FindOtelGraphs(context.Background(), lookups, reader)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(found) != len(lookups) {
		t.Fatalf("got %d calls and %d roots", calls, len(found))
	}
}

func graphFixture() (SpanLookup, []models.OtelSpan) {
	lookup := SpanLookup{ProjectId: uuid.New(), TraceId: NormalizeTraceId(uuid.NewString()), SpanId: "0102030405060708"}
	return lookup, []models.OtelSpan{
		{Span: models.Span{ProjectId: lookup.ProjectId, TraceId: lookup.TraceId, SpanId: lookup.SpanId}},
		{Span: models.Span{ProjectId: lookup.ProjectId, TraceId: lookup.TraceId, SpanId: "1112131415161718", ParentSpanId: lookup.SpanId}},
	}
}

func TestOtelGraphsRetainPartialTopology(t *testing.T) {
	lookup, topology := graphFixture()
	previous := MaxOtelGraphRows
	MaxOtelGraphRows = len(topology)
	t.Cleanup(func() { MaxOtelGraphRows = previous })
	reader := graphReader{topology: func(context.Context, []SpanLookup, uint64) ([]models.OtelSpan, error) {
		return append(topology, models.OtelSpan{}), nil
	}}
	found, err := FindOtelGraphs(context.Background(), []SpanLookup{lookup}, reader)
	if err != nil {
		t.Fatal(err)
	}
	graph := found[lookup.Owner()]
	if graph == nil || len(graph.Spans) != 1 || graph.Status.State != models.SpanGraphPartial || fmt.Sprint(graph.Status.Reasons) != "[row_limit]" {
		t.Fatalf("partial graph: %+v", graph)
	}
}

func TestOtelGraphsReadFailures(t *testing.T) {
	lookup, _ := graphFixture()
	for _, failure := range []error{context.DeadlineExceeded, errors.New("missing column")} {
		reader := graphReader{topology: func(context.Context, []SpanLookup, uint64) ([]models.OtelSpan, error) { return nil, failure }}
		found, err := FindOtelGraphs(context.Background(), []SpanLookup{lookup}, reader)
		if errors.Is(failure, context.DeadlineExceeded) {
			graph := found[lookup.Owner()]
			if err != nil || graph == nil || graph.Status.State != models.SpanGraphUnavailable || len(graph.Spans) != 0 {
				t.Fatalf("read limit: %+v, %v", graph, err)
			}
		} else if !errors.Is(err, failure) {
			t.Fatalf("storage failure hidden: %v", err)
		}
	}
}

func TestOtelGraphsPreserveTopologyWhenAttributesAreOmitted(t *testing.T) {
	lookup, topology := graphFixture()
	for _, reason := range []string{models.SpanGraphAttributeSize, models.SpanGraphAttributeBudget, models.SpanGraphAttributesUnavailable} {
		t.Run(reason, func(t *testing.T) {
			reader := graphReader{
				topology: func(context.Context, []SpanLookup, uint64) ([]models.OtelSpan, error) { return topology, nil },
				attributes: func(_ context.Context, _ SpanLookup, ids []string, limits OtelAttributeLimits) (map[string]OtelSpanAttributes, error) {
					if limits.PerSpanBytes != MaxOtelSpanAttributeBytes || limits.BudgetBytes != MaxOtelGraphBytes {
						t.Fatalf("limits: %+v", limits)
					}
					if reason == models.SpanGraphAttributesUnavailable {
						return nil, context.DeadlineExceeded
					}
					return map[string]OtelSpanAttributes{ids[0]: {Omitted: reason}}, nil
				},
			}
			found, err := FindOtelGraphs(context.Background(), []SpanLookup{lookup}, reader)
			if err != nil {
				t.Fatal(err)
			}
			graph := found[lookup.Owner()]
			if len(graph.Spans) != 1 || !graph.Spans[0].AttributesOmitted || graph.Status.State != models.SpanGraphPartial || graph.Status.OmittedAttributes != 1 || fmt.Sprint(graph.Status.Reasons) != "["+reason+"]" {
				t.Fatalf("omitted attributes: %+v", graph)
			}
		})
	}
}

func TestOtelGraphsSkipLookupsWithoutIds(t *testing.T) {
	reader := graphReader{topology: func(context.Context, []SpanLookup, uint64) ([]models.OtelSpan, error) {
		t.Fatal("a lookup that names no trace must not read spans")
		return nil, nil
	}}
	found, err := FindOtelGraphs(context.Background(), []SpanLookup{{ProjectId: uuid.New()}, {ProjectId: uuid.New(), TraceId: "0102030405060708090a0b0c0d0e0f10"}}, reader)
	if err != nil || len(found) != 0 {
		t.Fatalf("lookups without ids: %v %v", found, err)
	}
}

// A span whose parent never arrived shows under the trace's own root and nowhere else, and never crosses into another
// project or trace read in the same batch.
func TestOtelGraphOrphansBelongToTheirOwnRoot(t *testing.T) {
	project, other := uuid.New(), uuid.New()
	trace, otherTrace := "0102030405060708090a0b0c0d0e0f10", "1112131415161718191a1b1c1d1e1f20"
	span := func(project uuid.UUID, trace, id, parent string) models.OtelSpan {
		return models.OtelSpan{Span: models.Span{ProjectId: project, TraceId: trace, SpanId: id, ParentSpanId: parent}}
	}
	topology := []models.OtelSpan{
		span(project, trace, "aaaaaaaaaaaaaaa1", ""),
		span(project, trace, "aaaaaaaaaaaaaaa2", "aaaaaaaaaaaaaaa1"),
		span(project, trace, "aaaaaaaaaaaaaaa3", "00000000000000ff"),
		span(project, otherTrace, "bbbbbbbbbbbbbbb1", "00000000000000fe"),
		span(other, trace, "ccccccccccccccc1", "00000000000000fd"),
	}
	reader := graphReader{topology: func(context.Context, []SpanLookup, uint64) ([]models.OtelSpan, error) { return topology, nil }}
	root := SpanLookup{ProjectId: project, TraceId: trace, SpanId: "aaaaaaaaaaaaaaa1"}
	child := SpanLookup{ProjectId: project, TraceId: trace, SpanId: "aaaaaaaaaaaaaaa2"}
	found, err := FindOtelGraphs(context.Background(), []SpanLookup{root, child, {ProjectId: project, TraceId: otherTrace, SpanId: "bbbbbbbbbbbbbbb0"}, {ProjectId: other, TraceId: trace, SpanId: "ccccccccccccccc0"}}, reader)
	if err != nil {
		t.Fatal(err)
	}
	ids := func(graph *SpanGraph) string {
		out := []string{}
		for _, span := range graph.Spans {
			out = append(out, span.SpanId)
		}
		return fmt.Sprint(out)
	}
	if got := ids(found[root.Owner()]); got != "[aaaaaaaaaaaaaaa2 aaaaaaaaaaaaaaa3]" {
		t.Fatalf("the root takes its own trace's orphan and nothing else: %s", got)
	}
	if got := ids(found[child.Owner()]); got != "[]" {
		t.Fatalf("a span that is not the root takes no orphans: %s", got)
	}
}
