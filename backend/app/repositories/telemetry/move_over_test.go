package telemetry

import (
	"context"
	"errors"
	"fmt"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func paddedSpan(last byte) uuid.UUID {
	return uuid.UUID{8: last, 9: last, 10: last, 11: last, 12: last, 13: last, 14: last, 15: last}
}

func setupMoveOverProgress(t *testing.T) {
	t.Helper()
	path := "../../migrations/sqlite/0077_create_v2_move_over_days.up.sql"
	if db.Driver == lit.PostgreSQL {
		path = "../../migrations/pg/0143_create_v2_move_over_days.up.sql"
	}
	ddl, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(string(ddl)); err != nil {
		t.Fatal(err)
	}
	reset := func() {
		if _, err := db.DB.Exec("DELETE FROM v2_move_over_days"); err != nil {
			t.Fatal(err)
		}
	}
	reset()
	t.Cleanup(reset)
}

type failingMoveOverProgress struct {
	moveOverProgress
	failState string
}

var errProgressUnavailable = errors.New("progress store unavailable")

func (p *failingMoveOverProgress) SaveProgress(ctx context.Context, day transactional.MoveOverDay) error {
	if day.State == p.failState {
		p.failState = ""
		return errProgressUnavailable
	}
	return p.moveOverProgress.SaveProgress(ctx, day)
}

func TestMoveOverProgressFailureIsResumable(t *testing.T) {
	for _, failState := range []string{transactional.MoveOverStarted, transactional.MoveOverDone} {
		t.Run(failState, func(t *testing.T) {
			setupTestDB(t)
			setupMoveOverProgress(t)
			legacyReset(t)
			t.Cleanup(func() { legacyReset(t) })
			ctx := context.Background()
			at := time.Now().UTC().Truncate(moveOverDay).Add(10 * time.Hour)
			seedLegacyDay(t, uuid.New(), at, uuid.New(), "GET /retry")
			progress := &failingMoveOverProgress{failState: failState}
			m := &mover{MoveOverOptions: MoveOverOptions{PageSize: 2, Log: func(string, ...any) {}}, legacy: LegacyRepository, progress: progress}
			if err := m.run(ctx); !errors.Is(err, errProgressUnavailable) {
				t.Fatalf("expected injected progress failure: %v", err)
			}
			saved, err := progress.FindProgress(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if failState == transactional.MoveOverStarted {
				if len(saved) != 0 || moveOverRowCount(t, "endpoints_v2") != 0 || moveOverRowCount(t, "spans_v2") != 0 {
					t.Fatal("data must not be written without a durable started marker")
				}
			} else if len(saved) != 1 || saved[0].State != transactional.MoveOverStarted || saved[0].Table != "endpoints" {
				t.Fatalf("failed completion must leave the day resumable in the main DB: %+v", saved)
			}
			if err := m.run(ctx); err != nil {
				t.Fatal(err)
			}
			for table, want := range map[string]int64{"endpoints_v2": 1, "tasks_v2": 1, "exceptions_v2": 1, "spans_v2": 5, "endpoints": 1, "tasks": 1, "exception_stack_traces": 1, "spans": 3} {
				if got := moveOverRowCount(t, table); got != want {
					t.Errorf("%s physical rows after resume = %d, want %d", table, got, want)
				}
			}
			saved, err = progress.FindProgress(ctx)
			if err != nil || len(saved) != 4 {
				t.Fatalf("progress in main DB: %+v, %v", saved, err)
			}
			for _, day := range saved {
				wantRows := int64(1)
				if day.Table == "spans" {
					wantRows = 3
				}
				if day.State != transactional.MoveOverDone || day.Day != at.Format(time.DateOnly) || day.Rows != wantRows {
					t.Errorf("incomplete day: %+v", day)
				}
			}
		})
	}
}

func TestMoveOverIds(t *testing.T) {
	trace, browser, run := uuid.MustParse("01020304-0506-0708-090a-0b0c0d0e0f10"), uuid.MustParse("a1a2a3a4-a5a6-a7a8-a9aa-abacadaeaf00"), uuid.New()
	traceHex, browserHex := "0102030405060708090a0b0c0d0e0f10", "a1a2a3a4a5a6a7a8a9aaabacadaeaf00"

	for name, test := range map[string]struct {
		attributes  map[string]string
		distributed string
		id          uuid.UUID
		wantTrace   string
	}{
		"an OTel row names its trace in the attributes":    {map[string]string{"traceway.otel.trace_id": traceHex}, trace.String(), trace, traceHex},
		"the browser's id was filed as the distributed id": {map[string]string{"traceway.otel.trace_id": traceHex}, browser.String(), trace, traceHex},
		"an older OTel row only has the distributed id":    {nil, trace.String(), paddedSpan(7), traceHex},
		"a native run without a distributed trace":         {nil, "", run, hexId(run)},
		"a distributed id that is not an id is ignored":    {nil, "not-a-uuid", run, hexId(run)},
		"a zero distributed id is ignored":                 {nil, uuid.Nil.String(), run, hexId(run)},
		"a zero hex distributed id is ignored":             {nil, hexId(uuid.Nil), run, hexId(run)},
		"a zero OTel id falls back to the distributed id":  {map[string]string{traceIdAttribute: hexId(uuid.Nil)}, trace.String(), run, traceHex},
		"zero source ids fall back to the run":             {map[string]string{traceIdAttribute: hexId(uuid.Nil)}, uuid.Nil.String(), run, hexId(run)},
		"a zero distributed id does not become a link":     {map[string]string{traceIdAttribute: traceHex}, uuid.Nil.String(), run, traceHex},
	} {
		if gotTrace := traceIdFromLegacy(test.attributes, test.distributed, test.id); gotTrace != test.wantTrace {
			t.Errorf("%s: trace %q", name, gotTrace)
		}
	}
	if got := spanHex(paddedSpan(0xab).String()); got != "abababababababab" {
		t.Errorf("an OTel span id loses its zero padding: %q", got)
	}
	if got := spanHex(run.String()); got != hexId(run) {
		t.Errorf("a native span id stays whole: %q", got)
	}
	if got := spanHex(""); got != "" {
		t.Errorf("no id stays no id: %q", got)
	}

	recorded := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	task, span := mapTask(models.Task{Id: paddedSpan(5), TaskName: "settle", TraceId: trace.String(), SpanId: paddedSpan(5).String(), RecordedAt: recorded, Duration: time.Minute,
		Attributes: map[string]string{"traceway.otel.trace_id": traceHex, "messaging.system": "kafka"}})
	if task.TraceId != traceHex || task.SpanId != "0505050505050505" || !task.RecordedAt.Equal(recorded.Add(-time.Minute)) || len(task.Attributes) != 1 {
		t.Errorf("an OTel task moves to its start and sheds the identity attribute: %+v", task)
	}
	if span.SpanId != task.SpanId || span.TraceId != task.TraceId || span.SpanKind != spanKindConsumer || !span.StartTime.Equal(task.RecordedAt) {
		t.Errorf("the task's own span: %+v", span)
	}
	native, _ := mapTask(models.Task{Id: run, RecordedAt: recorded, Duration: time.Minute})
	if native.SpanId != hexId(run) || native.TraceId != hexId(run) || !native.RecordedAt.Equal(recorded) {
		t.Errorf("a native task was recorded at its start already: %+v", native)
	}

	project := uuid.New()
	owners := ownerIndex{}
	owners.add(project, trace, owner{"ffffffffffffffffffffffffffffffff", "f1f1f1f1f1f1f1f1", "yesterday", recorded.Add(-20 * time.Hour)})
	owners.add(project, trace, owner{traceHex, "e1e1e1e1e1e1e1e1", "gateway", recorded})
	onEndpoint := mapException(shared.LegacyException{ExceptionStackTrace: models.ExceptionStackTrace{ProjectId: project, TraceId: trace.String(), TraceType: "endpoint", RecordedAt: recorded.Add(time.Second)}, DistributedTraceId: browser.String()}, owners)
	if onEndpoint.TraceId != traceHex || onEndpoint.SpanId != "e1e1e1e1e1e1e1e1" || onEndpoint.TraceType != "endpoint" {
		t.Errorf("an exception lands on the span of the owner recorded nearest to it: %+v", onEndpoint)
	}
	unowned := mapException(shared.LegacyException{ExceptionStackTrace: models.ExceptionStackTrace{ProjectId: project, TraceId: browser.String(), TraceType: "task"}}, owners)
	if unowned.TraceId != browserHex || unowned.SpanId != "" || unowned.TraceType != "" {
		t.Errorf("an exception whose owner is gone keeps the trace and claims no kind: %+v", unowned)
	}
	inBrowser := mapException(shared.LegacyException{ExceptionStackTrace: models.ExceptionStackTrace{ProjectId: project, TraceType: "endpoint"}, DistributedTraceId: browser.String()}, owners)
	if inBrowser.TraceId != browserHex || inBrowser.SpanId != "" || inBrowser.TraceType != "" {
		t.Errorf("a browser exception belongs to the browser's trace: %+v", inBrowser)
	}

	child := mapSpan(shared.LegacySpan{ProjectId: project, Id: paddedSpan(9), OwnerId: trace, ParentSpanId: paddedSpan(8).String(), RecordedAt: recorded}, owners)
	if child.TraceId != traceHex || child.SpanId != "0909090909090909" || child.ParentSpanId != "0808080808080808" || child.ServiceName != "gateway" {
		t.Errorf("an OTel span keeps its own parent and takes its owner's trace: %+v", child)
	}
	nativeChild := mapSpan(shared.LegacySpan{ProjectId: project, Id: uuid.New(), OwnerId: run}, owners)
	if nativeChild.TraceId != hexId(run) || nativeChild.ParentSpanId != hexId(run) {
		t.Errorf("a native span without a parent hangs under its run: %+v", nativeChild)
	}
}

func TestMoveOverZeroDistributedIDKeepsNativeGraph(t *testing.T) {
	setupTestDB(t)
	setupMoveOverProgress(t)
	legacyReset(t)
	t.Cleanup(func() { legacyReset(t) })
	ctx := context.Background()
	at := time.Now().UTC().Truncate(moveOverDay).Add(10 * time.Hour)
	project, run, child, exception := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	legacyExec(t, `INSERT INTO endpoints (id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name, distributed_trace_id, span_id, is_stream, is_root)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, run, project, "GET /zero-header", int64(time.Second), at, 200, 0, "", "{}", "1.0", "api", &uuid.Nil, (*uuid.UUID)(nil), false, true)
	legacyExec(t, `INSERT INTO spans (id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, attributes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, child, run, project, "child", at, int64(time.Millisecond), at, (*uuid.UUID)(nil), "{}")
	legacyExec(t, `INSERT INTO exception_stack_traces (id, project_id, trace_id, trace_type, exception_hash, stack_trace, recorded_at, attributes, app_version, server_name, is_message, distributed_trace_id, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, exception, project, &run, "endpoint", "zero-header", "Error: zero header", at, "{}", "1.0", "api", false, &uuid.Nil, (*uuid.UUID)(nil))
	if err := RunMoveOver(ctx, MoveOverOptions{Log: func(string, ...any) {}}); err != nil {
		t.Fatal(err)
	}
	endpoint, err := EndpointRepository.FindById(ctx, project, run, &at)
	if err != nil || endpoint == nil || endpoint.TraceId != hexId(run) || endpoint.SpanId != hexId(run) {
		t.Fatalf("migrated endpoint: %+v %v", endpoint, err)
	}
	graph, err := SpanRepository.FindTrace(ctx, []uuid.UUID{project}, hexId(run), at)
	if err != nil || len(graph.Spans) != 2 {
		t.Fatalf("migrated root and child: %+v %v", graph, err)
	}
	for _, span := range graph.Spans {
		if span.TraceId != hexId(run) || (span.SpanId == hexId(child) && span.ParentSpanId != hexId(run)) {
			t.Fatalf("migrated span identity: %+v", span)
		}
	}
	exc, err := ExceptionStackTraceRepository.FindById(ctx, project, exception, &at)
	if err != nil || exc == nil || exc.TraceId != hexId(run) || exc.SpanId != hexId(run) {
		t.Fatalf("migrated exception: %+v %v", exc, err)
	}
	owner, err := FindExceptionOwner(ctx, *exc)
	if err != nil || owner == nil || owner.Name != endpoint.Endpoint {
		t.Fatalf("migrated exception owner: %+v %v", owner, err)
	}
}

// seedLegacyDay writes one request the way the released ingest stored it: a root endpoint filed under the trace id, a
// worker task below it filed under its span id, their spans filed under their owner, and an exception on the task.
func seedLegacyDay(t *testing.T, project uuid.UUID, at time.Time, trace uuid.UUID, endpointName string) {
	t.Helper()
	traceHex := hexId(trace)
	taskId, endpointSpan := paddedSpan(0x7a), paddedSpan(0xe1)
	attributes := fmt.Sprintf(`{"traceway.otel.trace_id":"%s","http.route":"/checkout"}`, traceHex)
	legacyExec(t, `INSERT INTO endpoints (id, project_id, endpoint, duration, recorded_at, status_code, body_size, client_ip, attributes, app_version, server_name, distributed_trace_id, span_id, is_stream, is_root)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, trace, project, endpointName, int64(time.Second), at, 500, 10, "10.0.0.1", attributes, "1.0.0", "gateway", &trace, &endpointSpan, false, true)
	legacyExec(t, `INSERT INTO tasks (id, project_id, task_name, duration, recorded_at, client_ip, attributes, app_version, server_name, distributed_trace_id, span_id, is_root)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, taskId, project, "settle", int64(time.Minute), at.Add(time.Minute), "", attributes, "1.0.0", "worker", &trace, &taskId, false)
	span := func(id, owner uuid.UUID, parent *uuid.UUID, name string) {
		legacyExec(t, `INSERT INTO spans (id, trace_id, project_id, name, start_time, duration, recorded_at, parent_span_id, attributes) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, owner, project, name, at, int64(time.Millisecond), at, parent, `{"db.system":"postgresql"}`)
	}
	query, publish := paddedSpan(0xb1), paddedSpan(0xc1)
	span(query, trace, &endpointSpan, "SELECT orders")
	span(publish, trace, &endpointSpan, "publish settle")
	span(paddedSpan(0x7b), taskId, &taskId, "UPDATE ledger")
	legacyExec(t, `INSERT INTO exception_stack_traces (id, project_id, trace_id, trace_type, exception_hash, stack_trace, recorded_at, attributes, app_version, server_name, is_message, distributed_trace_id, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, uuid.New(), project, &taskId, "task", "ledger-locked", "LockError: ledger", at.Add(time.Second), "{}", "1.0.0", "worker", false, &trace, (*uuid.UUID)(nil))
}

func TestMoveOverBringsHistoryIntoTheV2Tables(t *testing.T) {
	setupTestDB(t)
	setupMoveOverProgress(t)
	legacyReset(t)
	t.Cleanup(func() { legacyReset(t) })
	ctx := context.Background()
	project := uuid.New()
	today := time.Now().UTC().Truncate(24 * time.Hour).Add(10 * time.Hour)
	traces := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for age, trace := range traces {
		seedLegacyDay(t, project, today.AddDate(0, 0, -age), trace, fmt.Sprintf("GET /checkout/%d", age))
	}

	var moved []string
	options := MoveOverOptions{PageSize: 2, Oldest: today.AddDate(0, 0, -1), Log: func(format string, args ...any) {
		moved = append(moved, strings.SplitN(fmt.Sprintf(format, args...), ":", 2)[0])
	}}
	if err := RunMoveOver(ctx, options); err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, line := range moved {
		if strings.Contains(line, " endpoints") {
			order = append(order, strings.Fields(line)[0])
		}
	}
	if want := []string{today.Format(time.DateOnly), today.AddDate(0, 0, -1).Format(time.DateOnly)}; fmt.Sprint(order) != fmt.Sprint(want) {
		t.Fatalf("newest day first, and nothing older than asked for: %v", order)
	}

	window := today
	endpoints, err := EndpointRepository.FindByTraceIds(ctx, []string{hexId(traces[0])}, []uuid.UUID{project}, &window)
	if err != nil || len(endpoints) != 1 {
		t.Fatalf("moved endpoint: %+v %v", endpoints, err)
	}
	endpoint := endpoints[0]
	if endpoint.Id != traces[0] || endpoint.SpanId != "e1e1e1e1e1e1e1e1" || endpoint.ParentSpanId != "" || !endpoint.IsRoot ||
		endpoint.StatusCode != 500 || endpoint.ServerName != "gateway" || len(endpoint.Attributes) != 1 || endpoint.Attributes["http.route"] != "/checkout" {
		t.Fatalf("the endpoint keeps its row id and gains real ids, and the identity attribute is gone: %+v", endpoint)
	}
	graph, err := SpanRepository.FindGraph(ctx, shared.NewSpanLookup(project, endpoint.TraceId, endpoint.SpanId, endpoint.RecordedAt))
	if err != nil {
		t.Fatal(err)
	}
	names := func(spans []models.Span) string {
		out := make([]string, len(spans))
		for i, span := range spans {
			out[i] = span.Name
		}
		sort.Strings(out)
		return strings.Join(out, ", ")
	}
	if got := names(graph.Spans); got != "SELECT orders, publish settle" {
		t.Fatalf("the endpoint's waterfall survives the move: %q", got)
	}
	if graph.Spans[0].ServiceName != "gateway" || graph.Spans[0].Attributes["db.system"] != "postgresql" {
		t.Fatalf("spans take their owner's service and keep their attributes: %+v", graph.Spans[0])
	}

	tasks, err := TaskRepository.FindByTraceIds(ctx, []string{endpoint.TraceId}, []uuid.UUID{project}, &window)
	if err != nil || len(tasks) != 1 || tasks[0].SpanId != "7a7a7a7a7a7a7a7a" || !tasks[0].RecordedAt.Equal(today) {
		t.Fatalf("the task joins the endpoint's trace and is recorded at its start: %+v %v", tasks, err)
	}
	taskGraph, err := SpanRepository.FindGraph(ctx, shared.NewSpanLookup(project, tasks[0].TraceId, tasks[0].SpanId, tasks[0].RecordedAt))
	if err != nil || names(taskGraph.Spans) != "UPDATE ledger" {
		t.Fatalf("a span filed under a task that was not the root finds the trace through its owner: %+v %v", taskGraph, err)
	}
	exceptions, err := ExceptionStackTraceRepository.FindAllByTraceId(ctx, project, endpoint.TraceId, &window)
	if err != nil || len(exceptions) != 1 || exceptions[0].SpanId != tasks[0].SpanId || exceptions[0].TraceType != "task" {
		t.Fatalf("the exception lands on its task's span: %+v %v", exceptions, err)
	}
	owner, err := FindExceptionOwner(ctx, exceptions[0])
	if err != nil || owner == nil || owner.TraceType != "task" || owner.Name != "settle" {
		t.Fatalf("the issue page finds the task: %+v %v", owner, err)
	}
	whole, err := SpanRepository.FindTrace(ctx, []uuid.UUID{project}, endpoint.TraceId, today)
	if err != nil || names(whole.Spans) != fmt.Sprintf("%s, SELECT orders, UPDATE ledger, publish settle, settle", endpoint.Endpoint) {
		t.Fatalf("the whole trace holds the entities' own spans too: %q %v", names(whole.Spans), err)
	}
	if older, err := EndpointRepository.FindByTraceIds(ctx, []string{hexId(traces[2])}, []uuid.UUID{project}, nil); err != nil || len(older) != 0 {
		t.Fatalf("a day before the oldest asked for stays where it is: %+v %v", older, err)
	}

	count := func() int {
		found, err := EndpointRepository.FindByTraceIds(ctx, []string{hexId(traces[0]), hexId(traces[1]), hexId(traces[2])}, []uuid.UUID{project}, nil)
		if err != nil {
			t.Fatal(err)
		}
		whole, err := SpanRepository.FindTrace(ctx, []uuid.UUID{project}, hexId(traces[0]), today)
		if err != nil {
			t.Fatal(err)
		}
		return len(found)*100 + len(whole.Spans)
	}
	before := count()
	// A finished day is left alone.
	if err := RunMoveOver(ctx, options); err != nil || count() != before {
		t.Fatalf("running again must change nothing: %d -> %d %v", before, count(), err)
	}
	// A day that was interrupted is finished without writing its first rows twice.
	for _, table := range []string{"endpoints", "tasks", "exception_stack_traces", "spans"} {
		if err := (moveOverProgress{}).SaveProgress(ctx, transactional.MoveOverDay{Table: table, Day: today.Format(time.DateOnly), State: transactional.MoveOverStarted}); err != nil {
			t.Fatal(err)
		}
	}
	options.Oldest = time.Time{}
	if err := RunMoveOver(ctx, options); err != nil {
		t.Fatal(err)
	}
	if after := count(); after != before+100 {
		t.Fatalf("resuming adds the day that was left out and duplicates nothing: %d -> %d", before, after)
	}
	retried, err := ExceptionStackTraceRepository.FindAllByTraceId(ctx, project, endpoint.TraceId, &window)
	if err != nil || len(retried) != 1 {
		t.Fatalf("a resumed day must not write an exception twice: %d %v", len(retried), err)
	}
}
