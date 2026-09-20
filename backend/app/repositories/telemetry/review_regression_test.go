package telemetry

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestRegressionMoveOverPageKeyIncludesProject(t *testing.T) {
	setupTestDB(t)
	setupMoveOverProgress(t)
	legacyReset(t)
	t.Cleanup(func() { legacyReset(t) })
	at := time.Now().UTC().Truncate(moveOverDay).Add(10 * time.Hour)
	id := uuid.New()
	seedLegacyDay(t, uuid.New(), at, id, "GET /project-a")
	seedLegacyDay(t, uuid.New(), at, id, "GET /project-b")
	for _, project := range []uuid.UUID{uuid.New(), uuid.New()} {
		legacyExec(t, "INSERT INTO ai_traces (id, project_id, recorded_at, attributes) VALUES (?, ?, ?, ?)", id, project, at, "{}")
	}
	if err := RunMoveOver(context.Background(), MoveOverOptions{PageSize: 1, Log: func(string, ...any) {}}); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int64{"endpoints_v2": 2, "tasks_v2": 2, "ai_traces_v2": 2, "exceptions_v2": 2, "spans_v2": 12} {
		if got := moveOverRowCount(t, table); got != want {
			t.Fatalf("%s: got %d rows, want %d", table, got, want)
		}
	}
}

func TestRegressionMoveOverResumeKeyIncludesProject(t *testing.T) {
	setupTestDB(t)
	setupMoveOverProgress(t)
	legacyReset(t)
	t.Cleanup(func() { legacyReset(t) })
	ctx := context.Background()
	at := time.Now().UTC().Truncate(moveOverDay).Add(10 * time.Hour)
	id := uuid.New()
	first := uuid.New()
	seedLegacyDay(t, first, at, id, "GET /project-a")
	seedLegacyDay(t, uuid.New(), at, id, "GET /project-b")
	rows, err := LegacyRepository.FindEndpoints(ctx, at, at.Add(time.Second), 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ProjectId == first {
			entity, span := mapEndpoint(row)
			if err := SpanRepository.InsertAsync(ctx, []models.Span{span}); err != nil {
				t.Fatal(err)
			}
			if err := EndpointRepository.InsertAsync(ctx, []models.Endpoint{entity}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := (moveOverProgress{}).SaveProgress(ctx, transactional.MoveOverDay{Table: "endpoints", Day: at.Format(time.DateOnly), State: transactional.MoveOverStarted}); err != nil {
		t.Fatal(err)
	}
	if err := RunMoveOver(ctx, MoveOverOptions{PageSize: 100, Log: func(string, ...any) {}}); err != nil {
		t.Fatal(err)
	}
	if got := moveOverRowCount(t, "endpoints_v2"); got != 2 {
		t.Fatalf("resuming treats another project's ID as migrated: got %d endpoints, want 2", got)
	}
}

func TestRegressionSearchRetriesHaveUniqueUIKeys(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	span := canonicalSpan(uuid.New(), uuid.New(), uuid.New(), nil)
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{span, span}); err != nil {
		t.Fatal(err)
	}
	rows, total, err := OtelSpanRepository.Search(ctx, shared.OtelSpanSearch{ProjectId: span.ProjectId, From: span.StartTime.Add(-time.Hour), To: span.StartTime.Add(time.Hour), Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("duplicate retries changed search count: %d / %d", len(rows), total)
	}
	seen := map[string]bool{}
	for _, r := range rows {
		key := r.TraceId + ":" + r.SpanId
		if seen[key] {
			t.Fatalf("search returns duplicate Svelte row key %s: %d rows, total %d", key, len(rows), total)
		}
		seen[key] = true
	}
}

func TestRegressionGraphDuplicateAttributesMatchChosenSpan(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	root := uuid.New()
	span := canonicalSpan(uuid.New(), uuid.New(), uuid.New(), &root)
	span.Attributes = map[string]string{"version": "first"}
	best := span
	best.Duration = 2 * time.Second
	best.Attributes = map[string]string{"version": "longest"}
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{span, best}); err != nil {
		t.Fatal(err)
	}
	graph, err := SpanRepository.FindGraph(ctx, shared.SpanLookup{ProjectId: span.ProjectId, TraceId: span.TraceId, SpanId: span.ParentSpanId, RecordedAt: &span.StartTime})
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Spans) != 1 {
		t.Fatalf("graph rows: %d", len(graph.Spans))
	}
	if got := graph.Spans[0]; got.Duration != best.Duration || got.Attributes["version"] != "longest" {
		t.Fatalf("graph mixes duplicate versions: duration=%s attributes=%v; want 2s and longest", got.Duration, got.Attributes)
	}
}

func TestRegressionAttributeLimitCountsUTF8Bytes(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	span := canonicalSpan(uuid.New(), uuid.New(), uuid.New(), nil)
	span.Attributes = map[string]string{"large": strings.Repeat("界", 90000)}
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{span}); err != nil {
		t.Fatal(err)
	}
	attrs, err := OtelSpanRepository.FindSpanAttributes(ctx, shared.SpanLookup{ProjectId: span.ProjectId, TraceId: span.TraceId, RecordedAt: &span.StartTime}, []string{span.SpanId}, shared.OtelAttributeLimits{PerSpanBytes: 256 << 10, BudgetBytes: 32 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if got := attrs[span.SpanId]; got.Omitted == "" {
		t.Fatalf("accepted %d UTF-8 bytes over 262144-byte per-span cap", got.Bytes)
	}
}

func TestMoveOverRecoveryKeepsTraceAndOccurrenceIdentity(t *testing.T) {
	for _, table := range []string{"endpoints", "tasks", "ai_traces", "exception_stack_traces"} {
		for _, partial := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/span-only=%v", table, partial), func(t *testing.T) {
				setupTestDB(t)
				ctx := context.Background()
				at := time.Now().UTC().Truncate(time.Second)
				project, id := uuid.New(), uuid.New()
				traces := []uuid.UUID{uuid.New(), uuid.New()}
				m := &mover{legacy: LegacyRepository}
				write := func(firstOnly, resumed bool) error {
					var endpoints []models.Endpoint
					var tasks []models.Task
					var calls []models.AiTrace
					var exceptions []models.ExceptionStackTrace
					for i, trace := range traces {
						if firstOnly && i > 0 {
							break
						}
						for _, recorded := range []time.Time{at, at.Add(time.Millisecond)} {
							switch table {
							case "endpoints":
								endpoints = append(endpoints, models.Endpoint{ProjectId: project, Id: id, TraceId: trace.String(), RecordedAt: recorded, Duration: time.Second})
							case "tasks":
								tasks = append(tasks, models.Task{ProjectId: project, Id: id, TraceId: trace.String(), SpanId: paddedSpan(3).String(), RecordedAt: recorded, Duration: 8 * 24 * time.Hour})
							case "ai_traces":
								calls = append(calls, models.AiTrace{ProjectId: project, Id: id, TraceId: trace.String(), RecordedAt: recorded, Duration: time.Second, Attributes: map[string]string{}})
							case "exception_stack_traces":
								exceptions = append(exceptions, models.ExceptionStackTrace{ProjectId: project, Id: id, LinkedTraceId: trace.String(), RecordedAt: recorded, Attributes: map[string]string{}})
							}
						}
					}
					if firstOnly && partial && table != "exception_stack_traces" {
						var spans []models.Span
						for _, row := range endpoints {
							_, span := mapEndpoint(row)
							spans = append(spans, span)
						}
						for _, row := range tasks {
							_, span := mapTask(row)
							spans = append(spans, span)
						}
						for _, row := range calls {
							_, span := mapAiTrace(row)
							spans = append(spans, span)
						}
						return m.insertSpans(ctx, spans, at, at.Add(time.Second), false)
					}
					switch table {
					case "endpoints":
						return m.writeEndpoints(ctx, endpoints, at, at.Add(time.Second), resumed)
					case "tasks":
						return m.writeTasks(ctx, tasks, at, at.Add(time.Second), resumed)
					case "ai_traces":
						return m.writeAiTraces(ctx, calls, at, at.Add(time.Second), resumed)
					default:
						return m.writeExceptions(ctx, exceptions, at, at.Add(time.Second), resumed)
					}
				}
				if err := write(true, false); err != nil {
					t.Fatal(err)
				}
				if err := write(false, true); err != nil {
					t.Fatal(err)
				}
				if err := write(false, true); err != nil {
					t.Fatal(err)
				}
				if got := moveOverRowCount(t, shared.LegacyTables[table]); got != 4 {
					t.Fatalf("%s: got %d occurrences, want 4", table, got)
				}
			})
		}
	}
}

func TestSpanRetryWinnerMatchesGraphAttributesAndExport(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	root := uuid.New()
	first := canonicalSpan(uuid.New(), uuid.New(), uuid.New(), &root)
	first.Name, first.Attributes = "a-version", map[string]string{"version": "a"}
	last := first
	last.Name, last.Attributes = "z-version", map[string]string{"version": "z"}
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{last, first, last, first}); err != nil {
		t.Fatal(err)
	}
	lookup := shared.NewSpanLookup(first.ProjectId, first.TraceId, first.ParentSpanId, first.StartTime)
	graph, err := SpanRepository.FindGraph(ctx, lookup)
	if err != nil || len(graph.Spans) != 1 {
		t.Fatalf("graph: %+v, %v", graph, err)
	}
	payload, err := OtelSpanRepository.FindOTLP(ctx, first.ProjectId, first.TraceId, first.SpanId, first.StartTime)
	if err != nil {
		t.Fatal(err)
	}
	var envelope tracepb.ResourceSpans
	if err := proto.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	exported := envelope.ScopeSpans[0].Spans[0]
	attributes := shared.StringAttributes(exported.Attributes)
	if graph.Spans[0].Name != exported.Name || graph.Spans[0].Attributes["version"] != attributes["version"] {
		t.Fatalf("graph and export chose different retry versions: %+v / %v", graph.Spans[0], exported)
	}
	search := shared.OtelSpanSearch{ProjectId: first.ProjectId, From: first.StartTime.Add(-time.Hour), To: first.StartTime.Add(time.Hour), Page: 1, PageSize: 1}
	rows, total, err := OtelSpanRepository.Search(ctx, search)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].Name != exported.Name {
		t.Fatalf("search winner/count: %v %d %v", rows, total, err)
	}
	search.Page = 2
	rows, total, err = OtelSpanRepository.Search(ctx, search)
	if err != nil || total != 1 || len(rows) != 0 {
		t.Fatalf("retry leaked onto next page: %v %d %v", rows, total, err)
	}
	search.Page = 1
	discarded := "a"
	if attributes["version"] == discarded {
		discarded = "z"
	}
	search.Attributes = []shared.OtelAttributeFilter{{Key: "version", Value: discarded}}
	rows, total, err = OtelSpanRepository.Search(ctx, search)
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("filter matched a discarded retry: %v %d %v", rows, total, err)
	}
}

func TestAttributeBudgetCountsUTF8BytesAfterDeduplication(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	first := canonicalSpan(uuid.New(), uuid.New(), uuid.New(), nil)
	first.Attributes = map[string]string{"text": strings.Repeat("界", 200)}
	second := first
	second.SpanId = hexId(uuid.New())[16:]
	second.StartTime = first.StartTime.Add(time.Millisecond)
	if _, err := OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{first, first, second}); err != nil {
		t.Fatal(err)
	}
	lookup := shared.NewSpanLookup(first.ProjectId, first.TraceId, first.SpanId, first.StartTime)
	attributes, err := OtelSpanRepository.FindSpanAttributes(ctx, lookup, []string{first.SpanId, second.SpanId}, shared.OtelAttributeLimits{PerSpanBytes: 700, BudgetBytes: 900})
	if err != nil {
		t.Fatal(err)
	}
	if attributes[first.SpanId].Omitted != "" || attributes[second.SpanId].Omitted != models.SpanGraphAttributeBudget {
		t.Fatalf("incorrect UTF-8 budget or retry charged twice: %+v", attributes)
	}
	if attributes[first.SpanId].Bytes > 900 || attributes[second.SpanId].Bytes != 0 {
		t.Fatal("omitted data crossed the database boundary")
	}
}

func TestMoveOverRejectedSpanDoesNotCompleteDay(t *testing.T) {
	setupTestDB(t)
	setupMoveOverProgress(t)
	legacyReset(t)
	t.Cleanup(func() { legacyReset(t) })
	at := time.Now().UTC().Truncate(moveOverDay).Add(10 * time.Hour)
	seedLegacyDay(t, uuid.New(), at, uuid.Nil, "GET /invalid-trace")
	if err := RunMoveOver(context.Background(), MoveOverOptions{Log: func(string, ...any) {}}); err == nil {
		t.Fatal("rejected canonical span was reported as a successful migration")
	}
	progress, err := (moveOverProgress{}).FindProgress(context.Background())
	if err != nil || len(progress) != 1 || progress[0].State != transactional.MoveOverStarted {
		t.Fatalf("rejected day must stay started: %+v, %v", progress, err)
	}
	if moveOverRowCount(t, "endpoints_v2") != 0 {
		t.Fatal("rejected root must not leave an endpoint projection")
	}
}

func TestLegacySourceReadsHonorCancellation(t *testing.T) {
	setupTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	at := time.Now().UTC()
	if _, err := LegacyRepository.FindEndpoints(ctx, at.Add(-time.Hour), at, 10, nil); err == nil {
		t.Fatal("cancelled endpoint scan succeeded")
	}
	if _, err := LegacyRepository.FindTasks(ctx, at.Add(-time.Hour), at, 10, nil); err == nil {
		t.Fatal("cancelled task scan succeeded")
	}
	if _, err := LegacyRepository.FindAiTraces(ctx, at.Add(-time.Hour), at, 10, nil); err == nil {
		t.Fatal("cancelled AI scan succeeded")
	}
	if _, err := LegacyRepository.FindExceptions(ctx, at.Add(-time.Hour), at, 10, nil); err == nil {
		t.Fatal("cancelled exception scan succeeded")
	}
}
