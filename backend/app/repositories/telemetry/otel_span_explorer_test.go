package telemetry

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

func lookupUnder(span models.OtelSpan) shared.SpanLookup {
	return shared.NewSpanLookup(span.ProjectId, span.TraceId, span.SpanId, span.RecordedAt)
}

func spanNames(spans []models.Span) []string {
	names := make([]string, len(spans))
	for i, span := range spans {
		names[i] = span.Name
	}
	return names
}

func TestOtelGraphRootTakesOrphanedSubtrees(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project, trace := uuid.New(), uuid.New()
	rootID, childID, droppedID, orphanID, belowOrphanID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	named := func(name string, id uuid.UUID, parent *uuid.UUID) models.OtelSpan {
		span := canonicalSpan(project, trace, id, parent)
		span.Name = name
		return span
	}
	zeroPadded := func(id uuid.UUID) *uuid.UUID {
		parent := uuid.UUID{}
		copy(parent[8:], id[8:])
		return &parent
	}
	root, child := named("root", rootID, nil), named("child", childID, zeroPadded(rootID))
	// The span between the root and the orphan was dropped by a collector and never arrives.
	orphan, belowOrphan := named("orphan", orphanID, zeroPadded(droppedID)), named("below orphan", belowOrphanID, zeroPadded(orphanID))
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{root, child, orphan, belowOrphan}); err != nil {
		t.Fatal(err)
	}
	found, err := findSpans(ctx, root, &root.RecordedAt)
	if err != nil || len(found) != 3 {
		t.Fatalf("the trace root must show the subtree whose parent never arrived: %v %v", spanNames(found), err)
	}
	found, err = findSpans(ctx, child, &child.RecordedAt)
	if err != nil || len(found) != 0 {
		t.Fatalf("an occurrence below the root could be the wrong owner and must not take orphans: %v %v", spanNames(found), err)
	}
}

func TestOtelReadsAreBoundToTheLookupWindow(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project, trace := uuid.New(), uuid.New()
	rootID := uuid.New()
	parent := uuid.UUID{}
	copy(parent[8:], rootID[8:])
	root := canonicalSpan(project, trace, rootID, nil)
	near, far := canonicalSpan(project, trace, uuid.New(), &parent), canonicalSpan(project, trace, uuid.New(), &parent)
	near.Name, far.Name = "near", "three days later"
	far.StartTime = far.StartTime.Add(72 * time.Hour)
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{root, near, far}); err != nil {
		t.Fatal(err)
	}
	found, err := findSpans(ctx, root, &root.RecordedAt)
	if err != nil || len(found) != 1 || found[0].Name != "near" {
		t.Fatalf("a read is held to 24 hours either side of the occurrence: %v %v", spanNames(found), err)
	}
	if payload, err := OtelSpanRepository.FindOTLP(ctx, project, far.TraceId, far.SpanId, root.RecordedAt); err != nil || payload != nil {
		t.Fatalf("the payload read must use the same window: %d bytes %v", len(payload), err)
	}
	if payload, err := OtelSpanRepository.FindOTLP(ctx, project, far.TraceId, far.SpanId, far.StartTime); err != nil || payload == nil {
		t.Fatalf("anchored on its own time the span is found: %v", err)
	}
}

func TestOtelSpanSearch(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project, otherProject := uuid.New(), uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	request, job := uuid.New(), uuid.New()
	build := func(owner, trace uuid.UUID, name, service string, kind, status int32, duration time.Duration, start time.Time, attributes map[string]string) models.OtelSpan {
		span := canonicalSpan(owner, trace, uuid.New(), nil)
		span.Name, span.ServiceName, span.SpanKind, span.StatusCode, span.Duration, span.StartTime, span.Attributes = name, service, kind, status, duration, start, attributes
		return span
	}
	spans := []models.OtelSpan{
		build(project, request, "GET /users", "api", 2, 0, 50*time.Millisecond, now.Add(-time.Minute), map[string]string{"http.route": "/users"}),
		build(project, request, "SELECT users", "api", 3, 2, 5*time.Millisecond, now.Add(-50*time.Second), map[string]string{"db.system": "postgresql"}),
		build(project, job, "process job", "worker", 5, 0, 2*time.Second, now.Add(-30*time.Second), nil),
		build(project, uuid.New(), "three days ago", "api", 2, 0, time.Millisecond, now.Add(-72*time.Hour), nil),
		build(otherProject, uuid.New(), "another tenant", "api", 2, 0, time.Millisecond, now.Add(-time.Minute), nil),
	}
	if _, err := OtelSpanRepository.InsertAsync(ctx, spans); err != nil {
		t.Fatal(err)
	}
	kind, status := int32(5), int32(2)
	hour := shared.OtelSpanSearch{ProjectId: project, From: now.Add(-time.Hour), To: now.Add(time.Minute), Page: 1, PageSize: 50}
	with := func(change func(*shared.OtelSpanSearch)) shared.OtelSpanSearch {
		search := hour
		change(&search)
		return search
	}
	for _, tt := range []struct {
		name   string
		search shared.OtelSpanSearch
		want   []string
		total  uint64
	}{
		{"newest first, held to the range and the project", hour, []string{"process job", "SELECT users", "GET /users"}, 3},
		{"a wider range reaches older spans", with(func(s *shared.OtelSpanSearch) { s.From = now.Add(-7 * 24 * time.Hour) }), []string{"process job", "SELECT users", "GET /users", "three days ago"}, 4},
		{"service", with(func(s *shared.OtelSpanSearch) { s.Service = "worker" }), []string{"process job"}, 1},
		{"name, any case", with(func(s *shared.OtelSpanSearch) { s.Name = "select" }), []string{"SELECT users"}, 1},
		{"kind", with(func(s *shared.OtelSpanSearch) { s.Kind = &kind }), []string{"process job"}, 1},
		{"errors", with(func(s *shared.OtelSpanSearch) { s.Status = &status }), []string{"SELECT users"}, 1},
		{"slower than", with(func(s *shared.OtelSpanSearch) { s.MinDuration = time.Second }), []string{"process job"}, 1},
		{"faster than", with(func(s *shared.OtelSpanSearch) { s.MaxDuration = 10 * time.Millisecond }), []string{"SELECT users"}, 1},
		{"attribute with a dotted key", with(func(s *shared.OtelSpanSearch) {
			s.Attributes = []shared.OtelAttributeFilter{{Key: "http.route", Value: "/users"}}
		}), []string{"GET /users"}, 1},
		{"one trace", with(func(s *shared.OtelSpanSearch) { s.TraceId = spans[0].TraceId }), []string{"SELECT users", "GET /users"}, 2},
		{"slowest first", with(func(s *shared.OtelSpanSearch) { s.OrderBy = "duration desc" }), []string{"process job", "GET /users", "SELECT users"}, 3},
		{"second page", with(func(s *shared.OtelSpanSearch) { s.Page, s.PageSize = 2, 2 }), []string{"GET /users"}, 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			found, total, err := OtelSpanRepository.Search(ctx, tt.search)
			if err != nil {
				t.Fatal(err)
			}
			names := make([]string, len(found))
			for i, span := range found {
				names[i] = span.Name
			}
			if total != tt.total || len(names) != len(tt.want) {
				t.Fatalf("got %v of %d, want %v of %d", names, total, tt.want, tt.total)
			}
			for i := range names {
				if names[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", names, tt.want)
				}
			}
		})
	}
	first, _, err := OtelSpanRepository.Search(ctx, with(func(s *shared.OtelSpanSearch) { s.Service = "worker" }))
	if err != nil || len(first) != 1 || first[0].ServiceName != "worker" || first[0].SpanKind != 5 || first[0].Duration != 2*time.Second || !first[0].StartTime.Equal(spans[2].StartTime) {
		t.Fatalf("a result carries the fields the list shows: %+v %v", first, err)
	}
	services, err := OtelSpanRepository.Services(ctx, project, hour.From, hour.To)
	if err != nil || len(services) != 2 || services[0] != "api" || services[1] != "worker" {
		t.Fatalf("services in range, busiest first: %v %v", services, err)
	}
	trace, err := SpanRepository.FindTrace(ctx, []uuid.UUID{project}, spans[0].TraceId, now)
	if err != nil || len(trace.Spans) != 2 || trace.Status.State != models.SpanGraphComplete || trace.Spans[0].Name != "GET /users" {
		t.Fatalf("the whole trace, oldest first: %+v %v", trace, err)
	}
	if elsewhere, err := SpanRepository.FindTrace(ctx, []uuid.UUID{otherProject}, spans[0].TraceId, now); err != nil || len(elsewhere.Spans) != 0 {
		t.Fatalf("another project must not see the trace: %+v %v", elsewhere, err)
	}
}

func TestOtelWholeTraceKeepsTheMostImportantSpansPastTheCap(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	previous := shared.MaxOtelGraphRows
	shared.MaxOtelGraphRows = 5
	t.Cleanup(func() { shared.MaxOtelGraphRows = previous })

	project, trace := uuid.New(), uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	rootID := uuid.New()
	build := func(name string, id uuid.UUID, parent *uuid.UUID, kind, status int32, duration time.Duration, offset time.Duration) models.OtelSpan {
		span := canonicalSpan(project, trace, id, parent)
		span.Name, span.SpanKind, span.StatusCode, span.Duration, span.StartTime = name, kind, status, duration, now.Add(offset)
		return span
	}
	spans := []models.OtelSpan{
		build("root", rootID, nil, 1, 0, time.Second, 0),
		build("fast 1", uuid.New(), &rootID, 1, 0, time.Millisecond, time.Millisecond),
		build("fast 2", uuid.New(), &rootID, 1, 0, 2*time.Millisecond, 2*time.Millisecond),
		build("fast 3", uuid.New(), &rootID, 1, 0, 3*time.Millisecond, 3*time.Millisecond),
		build("slow", uuid.New(), &rootID, 1, 0, 400*time.Millisecond, 4*time.Millisecond),
		build("slower", uuid.New(), &rootID, 3, 0, 500*time.Millisecond, 5*time.Millisecond),
		build("consumer", uuid.New(), &rootID, 5, 0, time.Microsecond, 6*time.Millisecond),
		build("short error", uuid.New(), &rootID, 1, 2, time.Microsecond, 7*time.Millisecond),
	}
	if _, err := OtelSpanRepository.InsertAsync(ctx, spans); err != nil {
		t.Fatal(err)
	}
	graph, err := SpanRepository.FindTrace(ctx, []uuid.UUID{project}, spans[0].TraceId, now)
	if err != nil {
		t.Fatal(err)
	}
	// Cut by start time, the earliest five would be the root and the fast spans, and the error would be gone.
	want := []string{"root", "slow", "slower", "consumer", "short error"}
	if got := spanNames(graph.Spans); len(got) != len(want) {
		t.Fatalf("kept %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("kept %v, want %v in start order", got, want)
			}
		}
	}
	status := graph.Status
	if status.State != models.SpanGraphPartial || len(status.Reasons) != 2 || status.Reasons[0] != models.SpanGraphRowLimit || status.Reasons[1] != models.SpanGraphMostImportant {
		t.Fatalf("status %+v", status)
	}

	shared.MaxOtelGraphRows = previous
	whole, err := SpanRepository.FindTrace(ctx, []uuid.UUID{project}, spans[0].TraceId, now)
	if err != nil || len(whole.Spans) != len(spans) || whole.Status.State != models.SpanGraphComplete {
		t.Fatalf("a trace under the cap is returned whole: %d spans, %+v, %v", len(whole.Spans), whole.Status, err)
	}
	if whole.Spans[0].Name != "root" || whole.Spans[7].Name != "short error" {
		t.Fatalf("spans come back in start order whatever order they were read in: %v", spanNames(whole.Spans))
	}
}

func TestOtelWholeTraceLeavesAttributesToThePopover(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	project, trace := uuid.New(), uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	statement := "SELECT * FROM orders WHERE note = 'žž' AND " + strings.Repeat("id = 1 OR ", 40)
	query := canonicalSpan(project, trace, uuid.New(), nil)
	query.Name, query.StartTime, query.Attributes = "SELECT shop.orders", now, map[string]string{"db.query.text": statement, "db.system.name": "postgresql"}
	legacy := canonicalSpan(project, trace, uuid.New(), nil)
	legacy.Name, legacy.StartTime, legacy.Attributes = "old driver", now.Add(time.Millisecond), map[string]string{"db.statement": "DELETE FROM carts"}
	plain := canonicalSpan(project, trace, uuid.New(), nil)
	plain.Name, plain.StartTime = "no attributes", now.Add(2*time.Millisecond)
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{query, legacy, plain}); err != nil {
		t.Fatal(err)
	}

	graph, err := SpanRepository.FindTrace(ctx, []uuid.UUID{project}, query.TraceId, now)
	if err != nil || len(graph.Spans) != 3 {
		t.Fatalf("%v %v", graph, err)
	}
	for _, span := range graph.Spans {
		if len(span.Attributes) != 0 || span.AttributesOmitted {
			t.Fatalf("%s: the whole trace read carries no attributes: %+v", span.Name, span)
		}
	}
	preview := []rune(graph.Spans[0].DbStatement)
	if len(preview) != shared.OtelStatementPreviewLength || string(preview) != string([]rune(statement)[:shared.OtelStatementPreviewLength]) {
		t.Fatalf("statement preview is the first %d characters, got %d: %q", shared.OtelStatementPreviewLength, len(preview), graph.Spans[0].DbStatement)
	}
	if graph.Spans[1].DbStatement != "DELETE FROM carts" || graph.Spans[2].DbStatement != "" {
		t.Fatalf("db.statement is the fallback and other spans have none: %q %q", graph.Spans[1].DbStatement, graph.Spans[2].DbStatement)
	}

	loaded, err := SpanRepository.FindSpanAttributes(ctx, project, query.TraceId, query.SpanId, now)
	if err != nil || loaded == nil || loaded.Attributes["db.query.text"] != statement || loaded.Attributes["db.system.name"] != "postgresql" {
		t.Fatalf("popover attributes: %+v %v", loaded, err)
	}
	if empty, err := SpanRepository.FindSpanAttributes(ctx, project, plain.TraceId, plain.SpanId, now); err != nil || empty == nil || len(empty.Attributes) != 0 {
		t.Fatalf("a span without attributes is found and empty: %+v %v", empty, err)
	}
	if missing, err := SpanRepository.FindSpanAttributes(ctx, project, query.TraceId, "ffffffffffffffff", now); err != nil || missing != nil {
		t.Fatalf("unknown span: %+v %v", missing, err)
	}
	if outside, err := SpanRepository.FindSpanAttributes(ctx, project, query.TraceId, query.SpanId, now.Add(-72*time.Hour)); err != nil || outside != nil {
		t.Fatalf("outside the 24 hour window: %+v %v", outside, err)
	}
	if other, err := SpanRepository.FindSpanAttributes(ctx, uuid.New(), query.TraceId, query.SpanId, now); err != nil || other != nil {
		t.Fatalf("another project: %+v %v", other, err)
	}
}

func TestOtelGraphRootAdoptsOnlyItsOwnOrphans(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	gateway, warehouse, trace, otherTrace := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	named := func(project, trace uuid.UUID, name string, id uuid.UUID, parent *uuid.UUID) models.OtelSpan {
		span := canonicalSpan(project, trace, id, parent)
		span.Name = name
		return span
	}
	rootID, elsewhereID, unrelatedRootID, missingHop := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	root := named(gateway, trace, "GET /api/stock", rootID, nil)
	// Same trace, another project: its parent is a hop that reports to a third project, so it looks like an orphan.
	elsewhere := named(warehouse, trace, "GET /shelves", elsewhereID, &missingHop)
	unrelatedRoot := named(gateway, otherTrace, "GET /health", unrelatedRootID, nil)
	unrelatedOrphan := named(gateway, otherTrace, "lost child", uuid.New(), &missingHop)
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{root, elsewhere, unrelatedRoot, unrelatedOrphan}); err != nil {
		t.Fatal(err)
	}
	// The distributed trace card reads entities of several projects and traces in one batch.
	graphs, err := SpanRepository.FindGraphs(ctx, []shared.SpanLookup{
		lookupUnder(root),
		lookupUnder(elsewhere),
		lookupUnder(unrelatedRoot),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := spanNames(graphs[lookupUnder(root).Owner()].Spans); len(got) != 0 {
		t.Fatalf("a root must not adopt another project's or another trace's spans, got %v", got)
	}
	if got := spanNames(graphs[lookupUnder(unrelatedRoot).Owner()].Spans); len(got) != 1 || got[0] != "lost child" {
		t.Fatalf("a root still adopts the orphans of its own trace, got %v", got)
	}
}

func TestOtelWholeTraceReadsEveryProjectItIsGiven(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	gateway, inventory, warehouse, stranger, trace := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	hop := func(project uuid.UUID, name string, index int) models.OtelSpan {
		var parent *uuid.UUID
		if index > 0 {
			parent = &ids[index-1]
		}
		span := canonicalSpan(project, trace, ids[index], parent)
		span.Name, span.StartTime = name, now.Add(time.Duration(index)*time.Millisecond)
		return span
	}
	outsider := canonicalSpan(stranger, trace, uuid.New(), &ids[0])
	outsider.Name = "not one of the projects asked for"
	spans := []models.OtelSpan{hop(gateway, "GET /api/stock", 0), hop(gateway, "grpc client", 1), hop(inventory, "inventory.Stock/Get", 2),
		hop(inventory, "http client", 3), hop(warehouse, "GET /shelves", 4), outsider}
	if _, err := OtelSpanRepository.InsertAsync(ctx, spans); err != nil {
		t.Fatal(err)
	}

	whole, err := SpanRepository.FindTrace(ctx, []uuid.UUID{gateway, inventory, warehouse}, spans[0].TraceId, now)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /api/stock", "grpc client", "inventory.Stock/Get", "http client", "GET /shelves"}
	if got := spanNames(whole.Spans); len(got) != len(want) {
		t.Fatalf("one trace over three projects, in start order: got %v", got)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("one trace over three projects, in start order: got %v", got)
			}
		}
	}
	if whole.Spans[2].ProjectId != inventory || whole.Spans[4].ProjectId != warehouse {
		t.Fatalf("each span keeps the project it was reported to: %+v", whole.Spans)
	}
	// The distributed trace nests its entities from the same read, without attributes.
	parents, err := SpanRepository.FindTraceParents(ctx, []uuid.UUID{gateway, inventory, warehouse}, spans[0].TraceId, now)
	if err != nil || len(parents) != 4 || parents[spans[4].SpanId] != spans[3].SpanId || parents[spans[2].SpanId] != spans[1].SpanId {
		t.Fatalf("every hop points at its parent, across projects: %v %v", parents, err)
	}
	if _, isRoot := parents[spans[0].SpanId]; isRoot {
		t.Fatalf("the root has no parent: %v", parents)
	}
	if one, err := SpanRepository.FindTrace(ctx, []uuid.UUID{gateway}, spans[0].TraceId, now); err != nil || len(one.Spans) != 2 {
		t.Fatalf("a single project still reads only itself: %v %v", one, err)
	}
	if none, err := SpanRepository.FindTrace(ctx, nil, spans[0].TraceId, now); err != nil || len(none.Spans) != 0 || none.Status.State != models.SpanGraphComplete {
		t.Fatalf("no projects, no spans: %v %v", none, err)
	}
}
