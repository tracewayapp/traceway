package telemetry

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

const (
	backendTrace = "0102030405060708090a0b0c0d0e0f10"
	browserTrace = "a1a2a3a4a5a6a7a8a9aaabacadaeaf00"
	otherTrace   = "f1f2f3f4f5f6f7f8f9fafbfcfdfeff00"
)

// One request seen from a browser and three services, stored the way ingest stores it: every row keeps the trace id
// it arrived with. Retired browser correlation metadata must not affect lookups.
func seedTraceWithLegacyMetadata(t *testing.T, gateway, payments, stranger uuid.UUID, at time.Time) {
	t.Helper()
	ctx := context.Background()
	endpoint := func(project uuid.UUID, name, trace, span, parent, linked string, recordedAt time.Time) models.Endpoint {
		row := makeEndpoint(project, name, time.Millisecond, 200, recordedAt)
		row.TraceId, row.SpanId, row.ParentSpanId, row.IsRoot = trace, span, parent, parent == ""
		row.Attributes = map[string]string{"traceway.distributed_trace_id": linked}
		return row
	}
	if err := EndpointRepository.InsertAsync(ctx, []models.Endpoint{
		endpoint(gateway, "GET /checkout", backendTrace, "e100000000000001", "", browserTrace, at),
		endpoint(payments, "POST /charge", backendTrace, "e200000000000002", "c100000000000001", "", at.Add(time.Millisecond)),
		endpoint(gateway, "GET /unrelated", otherTrace, "e300000000000003", "", "", at),
		endpoint(stranger, "GET /not-yours", backendTrace, "e400000000000004", "", "", at),
		endpoint(gateway, "GET /long-ago", backendTrace, "e500000000000005", "", "", at.Add(-72*time.Hour)),
	}); err != nil {
		t.Fatal(err)
	}
	task := makeTask(payments, "settle", time.Millisecond, at.Add(2*time.Millisecond))
	task.TraceId, task.SpanId, task.ParentSpanId = backendTrace, "7a00000000000001", "e200000000000002"
	if err := TaskRepository.InsertAsync(ctx, []models.Task{task}); err != nil {
		t.Fatal(err)
	}
	ai := makeAiTrace(payments, "fraud-check", time.Millisecond, 10, 0.1, at.Add(3*time.Millisecond))
	ai.TraceId, ai.SpanId, ai.ParentSpanId, ai.IsRoot = backendTrace, "a100000000000001", "e200000000000002", false
	if err := AiTraceRepository.InsertAsync(ctx, []models.AiTrace{ai}); err != nil {
		t.Fatal(err)
	}
	exception := func(project uuid.UUID, hash, trace, span, kind, linked string) models.ExceptionStackTrace {
		row := makeException(project, hash, "Error: "+hash, at.Add(4*time.Millisecond))
		row.TraceId, row.SpanId, row.TraceType = trace, span, kind
		row.Attributes = map[string]string{"traceway.distributed_trace_id": linked}
		return row
	}
	message := exception(payments, "message", backendTrace, "e200000000000002", "endpoint", "")
	message.IsMessage = true
	if err := ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{
		exception(payments, "on-the-endpoint", backendTrace, "e200000000000002", "endpoint", ""),
		exception(payments, "below-the-endpoint", backendTrace, "5b00000000000001", "", ""),
		exception(gateway, "in-the-browser", browserTrace, "b000000000000001", "", ""),
		exception(gateway, "unrelated", otherTrace, "e300000000000003", "endpoint", ""),
		message,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFindByTraceIdsNeverFollowsLegacyLinks(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	gateway, payments, stranger := uuid.New(), uuid.New(), uuid.New()
	at := truncateMs(time.Now().UTC())
	seedTraceWithLegacyMetadata(t, gateway, payments, stranger, at)
	readable := []uuid.UUID{gateway, payments}

	names := func(endpoints []models.Endpoint) string {
		out := make([]string, len(endpoints))
		for i, endpoint := range endpoints {
			out[i] = endpoint.Endpoint
		}
		sort.Strings(out)
		return strings.Join(out, ", ")
	}

	byTrace, err := EndpointRepository.FindByTraceIds(ctx, []string{backendTrace}, readable, &at)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(byTrace); got != "GET /checkout, POST /charge" {
		t.Fatalf("the trace id finds every service's endpoint in the readable projects, inside the window: %s", got)
	}
	for _, endpoint := range byTrace {
		if endpoint.Endpoint == "GET /checkout" && (endpoint.SpanId != "e100000000000001" || endpoint.ParentSpanId != "" || !endpoint.IsRoot) {
			t.Fatalf("ids did not round trip: %+v", endpoint)
		}
		if endpoint.Endpoint == "POST /charge" && (endpoint.ParentSpanId != "c100000000000001" || endpoint.IsRoot) {
			t.Fatalf("ids did not round trip: %+v", endpoint)
		}
	}

	byLink, err := EndpointRepository.FindByTraceIds(ctx, []string{browserTrace}, readable, &at)
	if err != nil || len(byLink) != 0 {
		t.Fatalf("legacy metadata must not join separate traces: %s %v", names(byLink), err)
	}
	both, err := EndpointRepository.FindByTraceIds(ctx, []string{browserTrace, otherTrace}, readable, &at)
	if err != nil || names(both) != "GET /unrelated" {
		t.Fatalf("several ids in one read: %s %v", names(both), err)
	}
	unbounded, err := EndpointRepository.FindByTraceIds(ctx, []string{backendTrace}, readable, nil)
	if err != nil || names(unbounded) != "GET /checkout, GET /long-ago, POST /charge" {
		t.Fatalf("without a time the window is off: %s %v", names(unbounded), err)
	}
	for name, empty := range map[string]func() (int, error){
		"no trace ids": func() (int, error) {
			found, err := EndpointRepository.FindByTraceIds(ctx, nil, readable, &at)
			return len(found), err
		},
		"no projects": func() (int, error) {
			found, err := EndpointRepository.FindByTraceIds(ctx, []string{backendTrace}, nil, &at)
			return len(found), err
		},
	} {
		if count, err := empty(); err != nil || count != 0 {
			t.Fatalf("%s: %d %v", name, count, err)
		}
	}

	tasks, err := TaskRepository.FindByTraceIds(ctx, []string{backendTrace}, readable, &at)
	if err != nil || len(tasks) != 1 || tasks[0].SpanId != "7a00000000000001" || tasks[0].ParentSpanId != "e200000000000002" || tasks[0].TraceId != backendTrace {
		t.Fatalf("tasks: %+v %v", tasks, err)
	}
	aiTraces, err := AiTraceRepository.FindByTraceIds(ctx, []string{backendTrace}, readable, &at)
	if err != nil || len(aiTraces) != 1 || aiTraces[0].SpanId != "a100000000000001" || aiTraces[0].ParentSpanId != "e200000000000002" || aiTraces[0].TraceId != backendTrace {
		t.Fatalf("ai traces: %+v %v", aiTraces, err)
	}
	if none, err := TaskRepository.FindByTraceIds(ctx, nil, readable, &at); err != nil || len(none) != 0 {
		t.Fatalf("tasks without ids: %v %v", none, err)
	}
	if none, err := AiTraceRepository.FindByTraceIds(ctx, nil, readable, &at); err != nil || len(none) != 0 {
		t.Fatalf("ai traces without ids: %v %v", none, err)
	}
}

func TestExceptionsAreFoundByTraceAndKeepTheirSpan(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	gateway, payments, stranger := uuid.New(), uuid.New(), uuid.New()
	at := truncateMs(time.Now().UTC())
	seedTraceWithLegacyMetadata(t, gateway, payments, stranger, at)

	hashes := func(exceptions []models.ExceptionStackTrace) string {
		out := make([]string, len(exceptions))
		for i, exception := range exceptions {
			out[i] = exception.ExceptionHash
		}
		sort.Strings(out)
		return strings.Join(out, ", ")
	}

	card, err := ExceptionStackTraceRepository.FindByTraceIds(ctx, []string{backendTrace, browserTrace}, []uuid.UUID{gateway, payments}, &at)
	if err != nil || hashes(card) != "below-the-endpoint, in-the-browser, on-the-endpoint" {
		t.Fatalf("the card reads the exceptions of every id it knows, and no messages: %s %v", hashes(card), err)
	}
	if none, err := ExceptionStackTraceRepository.FindByTraceIds(ctx, nil, []uuid.UUID{gateway}, &at); err != nil || len(none) != 0 {
		t.Fatalf("no ids: %v %v", none, err)
	}

	page, err := ExceptionStackTraceRepository.FindAllByTraceId(ctx, payments, backendTrace, &at)
	if err != nil || hashes(page) != "below-the-endpoint, message, on-the-endpoint" {
		t.Fatalf("a detail page reads its own project's exceptions of the trace, messages included: %s %v", hashes(page), err)
	}
	for _, exception := range page {
		switch exception.ExceptionHash {
		case "on-the-endpoint":
			if exception.SpanId != "e200000000000002" || exception.TraceType != "endpoint" {
				t.Fatalf("span and kind did not round trip: %+v", exception)
			}
		case "below-the-endpoint":
			if exception.SpanId != "5b00000000000001" || exception.TraceType != "" {
				t.Fatalf("an exception below a promoted span keeps no kind: %+v", exception)
			}
		}
	}
	if elsewhere, err := ExceptionStackTraceRepository.FindAllByTraceId(ctx, gateway, backendTrace, &at); err != nil || len(elsewhere) != 0 {
		t.Fatalf("another project of the same trace holds none: %v %v", elsewhere, err)
	}
}

func TestFindExceptionOwner(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	gateway, payments, stranger := uuid.New(), uuid.New(), uuid.New()
	at := truncateMs(time.Now().UTC())
	seedTraceWithLegacyMetadata(t, gateway, payments, stranger, at)
	// POST /charge (e2) -> db span (5b) -> driver span (5c). The task hangs under e2 as well.
	span := func(id, parent string) models.Span {
		return models.Span{ProjectId: payments, TraceId: backendTrace, SpanId: id, ParentSpanId: parent, Name: id, StartTime: at, RecordedAt: at, Duration: time.Millisecond}
	}
	if err := SpanRepository.InsertAsync(ctx, []models.Span{span("e200000000000002", "c100000000000001"), span("5b00000000000001", "e200000000000002"),
		span("5c00000000000001", "5b00000000000001"), span("7a00000000000001", "e200000000000002"), span("7b00000000000001", "7a00000000000001")}); err != nil {
		t.Fatal(err)
	}
	exceptionOn := func(project uuid.UUID, trace, span string) models.ExceptionStackTrace {
		return models.ExceptionStackTrace{Id: uuid.New(), ProjectId: project, TraceId: trace, SpanId: span, RecordedAt: at}
	}
	for name, test := range map[string]struct {
		exception models.ExceptionStackTrace
		kind      string
		owner     string
	}{
		"on the endpoint's own span":         {exceptionOn(payments, backendTrace, "e200000000000002"), "endpoint", "POST /charge"},
		"two spans below the endpoint":       {exceptionOn(payments, backendTrace, "5c00000000000001"), "endpoint", "POST /charge"},
		"below the task, the nearest owner":  {exceptionOn(payments, backendTrace, "7b00000000000001"), "task", "settle"},
		"on a span that was never stored":    {exceptionOn(payments, backendTrace, "ffffffffffffffff"), "", ""},
		"in a project with no such trace":    {exceptionOn(stranger, otherTrace, "e300000000000003"), "", ""},
		"in the browser, linked but no span": {exceptionOn(gateway, browserTrace, "b000000000000001"), "", ""},
		"without a trace":                    {exceptionOn(payments, "", ""), "", ""},
	} {
		owner, err := FindExceptionOwner(ctx, test.exception)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if test.kind == "" {
			if owner != nil {
				t.Fatalf("%s: expected no owner, got %+v", name, owner)
			}
			continue
		}
		if owner == nil || owner.TraceType != test.kind || owner.Name != test.owner {
			t.Fatalf("%s: got %+v", name, owner)
		}
	}
}
