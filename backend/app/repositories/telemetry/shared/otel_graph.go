package shared

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	traceway "go.tracewayapp.com"
)

var (
	MaxOtelGraphRows          = 20000
	MaxOtelGraphBytes         = 32 << 20
	MaxOtelSpanAttributeBytes = 256 << 10
)

const (
	maxAnchoredTopologyReads = 8
	otelAttributeChunk       = 4000
)

const OtelTopologyColumns = `project_id, trace_id, span_id, parent_span_id, name, duration, span_kind, status_code, service_name, scope_name, start_time_unix_nano`

type OtelAttributeLimits struct{ PerSpanBytes, BudgetBytes int }

type OtelSpanAttributes struct {
	Attributes map[string]string
	Omitted    string
	Bytes      int
}

type OtelGraphReader interface {
	// FindTraceTopology returns scalar span rows in (start, span ID) order, at most MaxOtelGraphRows+1 of them.
	FindTraceTopology(ctx context.Context, lookups []SpanLookup, fromUnixNano uint64) ([]models.OtelSpan, error)
	// FindTraceOutline returns one whole trace across the projects of the lookups, most important spans first, at most MaxOtelGraphRows+1 rows, each with its statement preview and no attributes.
	FindTraceOutline(ctx context.Context, lookups []SpanLookup) ([]models.OtelSpan, error)
	FindSpanAttributes(ctx context.Context, lookup SpanLookup, spanIds []string, limits OtelAttributeLimits) (map[string]OtelSpanAttributes, error)
	IsReadLimitError(err error) bool
}

type SpanGraph struct {
	Spans  []models.Span
	Status models.SpanGraphStatus
}

// OtelAttributeQuery applies both byte limits inside the database, so an oversized value is never transferred.
func OtelAttributeQuery(attributes, startOrder, predicate string, limits OtelAttributeLimits) string {
	return fmt.Sprintf(`SELECT span_id,
		CASE WHEN size > %[1]d OR running > %[2]d THEN '' ELSE attributes END,
		CASE WHEN size > %[1]d THEN 2 WHEN running > %[2]d THEN 1 ELSE 0 END
		FROM (SELECT span_id, attributes, size, start_order,
			SUM(CASE WHEN size > %[1]d THEN 0 ELSE size END) OVER (ORDER BY start_order, span_id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS running
			FROM (SELECT span_id, %[3]s AS attributes, length(%[3]s) AS size, %[4]s AS start_order FROM `+SpansTable+` WHERE %[5]s) sized) budgeted
		ORDER BY start_order, span_id`, limits.PerSpanBytes, limits.BudgetBytes, attributes, startOrder, predicate)
}

func OtelAttributeOmission(flag int) string {
	switch flag {
	case 2:
		return models.SpanGraphAttributeSize
	case 1:
		return models.SpanGraphAttributeBudget
	}
	return ""
}

type otelGraphIndex struct {
	byId     map[string]models.OtelSpan
	children map[string][]string
}

func otelSpanKey(project uuid.UUID, traceId, spanId string) string {
	return project.String() + ":" + traceId + ":" + spanId
}

func newOtelGraphIndex(topology []models.OtelSpan) otelGraphIndex {
	index := otelGraphIndex{make(map[string]models.OtelSpan, len(topology)), make(map[string][]string)}
	for _, span := range topology {
		key := otelSpanKey(span.ProjectId, span.TraceId, span.SpanId)
		// Export retries are append-only. Choose the same completed span regardless of arrival order.
		if old, exists := index.byId[key]; !exists || preferOtelSpan(span, old) {
			index.byId[key] = span
		}
	}
	for key, span := range index.byId {
		if span.ParentSpanId == "" {
			continue
		}
		if parentKey := otelSpanKey(span.ProjectId, span.TraceId, span.ParentSpanId); parentKey != key {
			index.children[parentKey] = append(index.children[parentKey], key)
		}
	}
	return index
}

// subtree returns the spans under the span the lookup names, without that span itself.
func (index otelGraphIndex) subtree(lookup SpanLookup) []models.Span {
	rootKey := otelSpanKey(lookup.ProjectId, lookup.TraceId, lookup.SpanId)
	spans := []models.Span{}
	visited := map[string]bool{rootKey: true}
	pending := append([]string{}, index.children[rootKey]...)
	if root, stored := index.byId[rootKey]; stored && root.ParentSpanId == "" {
		// A span whose parent never arrived, because a collector dropped it, would hide everything below it. The
		// trace's own root takes those subtrees. Any other span could be the wrong owner, so it does not. A batch can
		// hold other projects and other traces, so only this trace's own orphans belong to its root.
		for parentKey, orphans := range index.children {
			if _, arrived := index.byId[parentKey]; arrived {
				continue
			}
			for _, orphan := range orphans {
				if span := index.byId[orphan]; span.ProjectId == root.ProjectId && span.TraceId == root.TraceId {
					pending = append(pending, orphan)
				}
			}
		}
	}
	for len(pending) > 0 {
		key := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if visited[key] {
			continue
		}
		visited[key] = true
		spans = append(spans, index.byId[key].Span)
		pending = append(pending, index.children[key]...)
	}
	sortOtelSpans(spans)
	return spans
}

func sortOtelSpans(spans []models.Span) {
	slices.SortFunc(spans, func(a, b models.Span) int {
		if c := a.StartTime.Compare(b.StartTime); c != 0 {
			return c
		}
		return strings.Compare(a.SpanId, b.SpanId)
	})
}

func addReason(status *models.SpanGraphStatus, state, reason string) {
	if status.State != models.SpanGraphUnavailable {
		status.State = state
	}
	if !slices.Contains(status.Reasons, reason) {
		status.Reasons = append(status.Reasons, reason)
	}
}

func readLimitReached(ctx context.Context, reader OtelGraphReader, err error) bool {
	// The client went away; nobody reads the result and there is nothing to report.
	if errors.Is(ctx.Err(), context.Canceled) {
		return true
	}
	if !errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil && !reader.IsReadLimitError(err) {
		return false
	}
	traceway.CaptureException(fmt.Errorf("otel span graph read hit a limit: %w", err))
	return true
}

// FindOtelGraphs reads topology first and attributes second. A limit never fails the caller: the graph comes back
// partial or unavailable and says why. A lookup that names no trace or no span is skipped.
func FindOtelGraphs(ctx context.Context, lookups []SpanLookup, reader OtelGraphReader) (map[SpanOwner]*SpanGraph, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result := make(map[SpanOwner]*SpanGraph)
	budget, anchoredReads := MaxOtelGraphBytes, 0
	for batch := range slices.Chunk(lookups, SpanLookupBatchSize) {
		traceKeys := make(map[string]bool)
		traces, wanted := make([]SpanLookup, 0, len(batch)), make([]SpanLookup, 0, len(batch))
		for _, lookup := range batch {
			if lookup.TraceId == "" || lookup.SpanId == "" {
				continue
			}
			wanted = append(wanted, lookup)
			if key := OtelKey(lookup.ProjectId, lookup.TraceId); !traceKeys[key] {
				traceKeys[key] = true
				traces = append(traces, lookup)
			}
		}
		if len(traces) == 0 {
			continue
		}
		topology, err := reader.FindTraceTopology(ctx, traces, 0)
		if err != nil {
			if !readLimitReached(ctx, reader, err) {
				return nil, err
			}
			for _, lookup := range wanted {
				result[lookup.Owner()] = &SpanGraph{Spans: []models.Span{}, Status: models.SpanGraphStatus{State: models.SpanGraphUnavailable, Reasons: []string{models.SpanGraphReadLimit}}}
			}
			continue
		}
		truncated := len(topology) > MaxOtelGraphRows
		index := newOtelGraphIndex(topology[:min(len(topology), MaxOtelGraphRows)])
		for _, lookup := range wanted {
			if _, exists := result[lookup.Owner()]; exists {
				continue
			}
			graph := &SpanGraph{Status: models.SpanGraphStatus{State: models.SpanGraphComplete}}
			graph.Spans = index.subtree(lookup)
			if truncated {
				addReason(&graph.Status, models.SpanGraphPartial, models.SpanGraphRowLimit)
				// An unanchored cap keeps the earliest spans of the trace, which can exclude a span that starts late.
				if lookup.StartUnixNano > 0 && anchoredReads < maxAnchoredTopologyReads {
					anchoredReads++
					anchored, err := reader.FindTraceTopology(ctx, []SpanLookup{lookup}, lookup.StartUnixNano)
					if err != nil && !readLimitReached(ctx, reader, err) {
						return nil, err
					}
					if err == nil {
						if spans := newOtelGraphIndex(anchored[:min(len(anchored), MaxOtelGraphRows)]).subtree(lookup); len(spans) > len(graph.Spans) {
							graph.Spans = spans
						}
					}
				}
			}
			if err := loadOtelAttributes(ctx, reader, lookup, graph, &budget); err != nil {
				return nil, err
			}
			result[lookup.Owner()] = graph
		}
	}
	return result, nil
}

// FindOtelTrace reads a whole trace for the explorer, one lookup per project the caller may read. Attributes stay behind: the page asks for one span's attributes
// when its popover opens, so a 20,000 span trace is a few megabytes and not the full attribute budget.
func FindOtelTrace(ctx context.Context, lookups []SpanLookup, reader OtelGraphReader) (*SpanGraph, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	graph := &SpanGraph{Spans: []models.Span{}, Status: models.SpanGraphStatus{State: models.SpanGraphComplete}}
	if len(lookups) == 0 {
		return graph, nil
	}
	outline, err := reader.FindTraceOutline(ctx, lookups)
	if err != nil {
		if !readLimitReached(ctx, reader, err) {
			return nil, err
		}
		graph.Status = models.SpanGraphStatus{State: models.SpanGraphUnavailable, Reasons: []string{models.SpanGraphReadLimit}}
		return graph, nil
	}
	if len(outline) > MaxOtelGraphRows {
		addReason(&graph.Status, models.SpanGraphPartial, models.SpanGraphRowLimit)
		addReason(&graph.Status, models.SpanGraphPartial, models.SpanGraphMostImportant)
		outline = outline[:MaxOtelGraphRows]
	}
	for _, span := range newOtelGraphIndex(outline).byId {
		graph.Spans = append(graph.Spans, span.Span)
	}
	sortOtelSpans(graph.Spans)
	return graph, nil
}

// FindOtelTraceParents maps every span of a trace to its parent, across the projects of the lookups. It reads the
// topology columns and leaves the attributes on disk. Entities of a trace are joined by the spans between them, and those spans
// need not belong to any entity: a gRPC hop reporting to its own project is one. A trace the read limits cut short
// yields what was read.
func FindOtelTraceParents(ctx context.Context, lookups []SpanLookup, reader OtelGraphReader) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	parents := map[string]string{}
	if len(lookups) == 0 {
		return parents, nil
	}
	topology, err := reader.FindTraceTopology(ctx, lookups, 0)
	if err != nil {
		if readLimitReached(ctx, reader, err) {
			return parents, nil
		}
		return nil, err
	}
	for _, span := range topology {
		if span.ParentSpanId != "" {
			parents[span.SpanId] = span.ParentSpanId
		}
	}
	return parents, nil
}

// FindOtelSpanAttributes loads the attributes of one span under the same per-span size limit as a graph read. It
// returns nil when the span is not stored inside the lookup window.
func FindOtelSpanAttributes(ctx context.Context, lookup SpanLookup, spanId string, reader OtelGraphReader) (*OtelSpanAttributes, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	loaded, err := reader.FindSpanAttributes(ctx, lookup, []string{spanId}, OtelAttributeLimits{MaxOtelSpanAttributeBytes, MaxOtelGraphBytes})
	if err != nil {
		if readLimitReached(ctx, reader, err) {
			return &OtelSpanAttributes{Omitted: models.SpanGraphAttributesUnavailable}, nil
		}
		return nil, err
	}
	if found, ok := loaded[spanId]; ok {
		return &found, nil
	}
	return nil, nil
}

func loadOtelAttributes(ctx context.Context, reader OtelGraphReader, lookup SpanLookup, graph *SpanGraph, budget *int) error {
	positions := make(map[string][]int, len(graph.Spans))
	ids := make([]string, 0, len(graph.Spans))
	for i, span := range graph.Spans {
		if _, seen := positions[span.SpanId]; !seen {
			ids = append(ids, span.SpanId)
		}
		positions[span.SpanId] = append(positions[span.SpanId], i)
	}
	omit := func(spanIds []string, reason string) {
		for _, id := range spanIds {
			for _, i := range positions[id] {
				graph.Spans[i].AttributesOmitted = true
				graph.Status.OmittedAttributes++
			}
		}
		if len(spanIds) > 0 {
			addReason(&graph.Status, models.SpanGraphPartial, reason)
		}
	}
	for chunk := range slices.Chunk(ids, otelAttributeChunk) {
		if *budget <= 0 {
			omit(chunk, models.SpanGraphAttributeBudget)
			continue
		}
		loaded, err := reader.FindSpanAttributes(ctx, lookup, chunk, OtelAttributeLimits{MaxOtelSpanAttributeBytes, *budget})
		if err != nil {
			if !readLimitReached(ctx, reader, err) {
				return err
			}
			omit(chunk, models.SpanGraphAttributesUnavailable)
			continue
		}
		for _, id := range chunk {
			attributes := loaded[id]
			if attributes.Omitted != "" {
				omit([]string{id}, attributes.Omitted)
				continue
			}
			*budget -= attributes.Bytes
			for _, i := range positions[id] {
				graph.Spans[i].Attributes = attributes.Attributes
			}
		}
	}
	return nil
}

// SpanOwner keys a graph by the span it hangs under.
type SpanOwner struct {
	ProjectId       uuid.UUID
	TraceId, SpanId string
}

func (lookup SpanLookup) Owner() SpanOwner {
	return SpanOwner{lookup.ProjectId, lookup.TraceId, lookup.SpanId}
}
