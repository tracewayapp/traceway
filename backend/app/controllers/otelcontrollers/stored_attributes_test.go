package otelcontrollers

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/v2/app/services"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

const canonicalTestSession = "0195f2a0-0000-7000-8000-00000000beef"

func TestConvertLogs_SessionIdIsStoredCanonically(t *testing.T) {
	setFakeStore(t, nil)
	projectId := uuid.New()
	at := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	for _, spelling := range []string{
		"0195F2A0-0000-7000-8000-00000000BEEF",
		"0195f2a000007000800000000000beef",
		"{0195f2a0-0000-7000-8000-00000000beef}",
		"urn:uuid:0195f2a0-0000-7000-8000-00000000beef",
	} {
		req := eventLogs(browserResource(strKV(attrSessionId, spelling)),
			eventRecord("browser.console", at, strKV(attrSessionId, spelling)),
			eventRecord(eventException, at, strKV(attrSessionId, spelling), strKV("exception.type", "Error"), strKV("exception.message", "boom")),
		)
		result := convertLogs(context.Background(), browserProject(projectId), projectId, "", req)
		for _, record := range result.Records {
			if record.LogAttributes[attrSessionId] != canonicalTestSession || record.ResourceAttributes[attrSessionId] != canonicalTestSession {
				t.Errorf("%s: stored session ids = %q (record), %q (resource), want %s", spelling,
					record.LogAttributes[attrSessionId], record.ResourceAttributes[attrSessionId], canonicalTestSession)
			}
		}
		if len(result.Exceptions) != 1 || result.Exceptions[0].Attributes[attrSessionId] != canonicalTestSession ||
			result.Exceptions[0].SessionId == nil || result.Exceptions[0].SessionId.String() != canonicalTestSession {
			t.Errorf("%s: exception session = %+v", spelling, result.Exceptions)
		}
	}

	req := eventLogs(browserResource(), eventRecord("browser.console", at, strKV(attrSessionId, "not-a-uuid")))
	if got := convertLogs(context.Background(), browserProject(projectId), projectId, "", req).Records[0].LogAttributes[attrSessionId]; got != "not-a-uuid" {
		t.Errorf("a session id that is not a UUID must be kept as sent, got %q", got)
	}
}

func TestConvertLogs_EventNameAttributeNamesTheEvent(t *testing.T) {
	setFakeStore(t, nil)
	projectId := uuid.New()
	at := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	started := at.Add(-time.Minute)
	attributeEvent := func(name string, attrs ...*commonpb.KeyValue) *logspb.LogRecord {
		return eventRecord("", at, append(attrs, strKV(attrEventName, name))...)
	}
	req := eventLogs(browserResource(),
		attributeEvent(eventException, strKV("exception.type", "TypeError"), strKV("exception.message", "x is undefined")),
		attributeEvent(eventMessage, strKV(attrSessionId, canonicalTestSession)),
		attributeEvent(eventSessionEnd, strKV(attrSessionId, canonicalTestSession), intKV(attrSessionStarted, started.UnixMilli())),
	)
	req.ResourceLogs[0].ScopeLogs[0].LogRecords[1].Body = &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "checkout opened"}}

	result := convertLogs(context.Background(), browserProject(projectId), projectId, "", req)
	if len(result.Exceptions) != 2 {
		t.Fatalf("got %d issues, want the exception and the message", len(result.Exceptions))
	}
	if result.Exceptions[0].IsMessage || !strings.HasPrefix(result.Exceptions[0].StackTrace, "TypeError: x is undefined") {
		t.Errorf("exception = %+v", result.Exceptions[0])
	}
	if !result.Exceptions[1].IsMessage || result.Exceptions[1].StackTrace != "checkout opened" {
		t.Errorf("message = %+v", result.Exceptions[1])
	}
	if len(result.Sessions) != 1 || result.Sessions[0].EndedAt == nil || !result.Sessions[0].StartedAt.Equal(started) {
		t.Fatalf("sessions = %+v, want one ended session", result.Sessions)
	}
	if _, ok := result.Sessions[0].Attributes[attrEventName]; ok {
		t.Errorf("event.name must not be a session attribute: %v", result.Sessions[0].Attributes)
	}
}

func TestConvertLogs_DebugIdsOnTheResourceAreNotStored(t *testing.T) {
	projectId := uuid.New()
	setFakeStore(t, map[string][]byte{
		services.SourceMapStorageKey(projectId, services.DebugIdMapName(testDebugId)): []byte(testSourceMap),
	})
	at := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	req := eventLogs(browserResource(strKV(attrTracewayDebugIds, `{"app-4f2a.js":"`+testDebugId+`"}`)),
		eventRecord(eventException, at,
			strKV("exception.type", "TypeError"),
			strKV("exception.message", "x is undefined"),
			strKV("exception.stacktrace", "TypeError: x is undefined\n    at t (https://cdn.example.com/assets/app-4f2a.js:1:11)"),
		))
	for name, project := range map[string]*models.Project{"browser": browserProject(projectId), "backend": tokenProject(projectId)} {
		result := convertLogs(context.Background(), project, projectId, "", req)
		record := result.Records[0]
		if _, ok := record.ResourceAttributes[attrTracewayDebugIds]; ok {
			t.Errorf("%s: debug ids stored in the log row's resource attributes", name)
		}
		if !strings.Contains(record.LogAttributes["exception.stacktrace"], "original.js") {
			t.Errorf("%s: the resource's debug ids must still symbolicate: %q", name, record.LogAttributes["exception.stacktrace"])
		}
		if _, ok := result.Exceptions[0].Attributes[attrTracewayDebugIds]; ok {
			t.Errorf("%s: debug ids stored on the issue", name)
		}
	}
}

// A non-browser project keeps the span's attributes on its exception and endpoint rows, and every project keeps the
// span itself; none of those readable copies may carry the debug ids, while the lossless payload keeps them.
func TestConvertTraces_DebugIdsAndSessionIdInStoredAttributes(t *testing.T) {
	projectId := uuid.New()
	setFakeStore(t, map[string][]byte{
		services.SourceMapStorageKey(projectId, services.DebugIdMapName(testDebugId)): []byte(testSourceMap),
	})
	debugIds := `{"app-4f2a.js":"` + testDebugId + `"}`
	now := uint64(time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC).UnixNano())
	req := &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
			strKV("service.name", "ssr"), strKV("telemetry.sdk.language", "nodejs"), strKV(attrTracewayDebugIds, debugIds),
			strKV(attrSessionId, "0195F2A0-0000-7000-8000-00000000BEEF"),
		}},
		ScopeSpans: []*tracepb.ScopeSpans{{
			Scope: &commonpb.InstrumentationScope{Name: "@opentelemetry/instrumentation-http"},
			Spans: []*tracepb.Span{{
				TraceId: bytes.Repeat([]byte{0xa1}, 16), SpanId: bytes.Repeat([]byte{0xb2}, 8),
				Name: "GET /cart", Kind: tracepb.Span_SPAN_KIND_SERVER, StartTimeUnixNano: now, EndTimeUnixNano: now + 1_000_000,
				Attributes: []*commonpb.KeyValue{strKV("http.request.method", "GET"), strKV("http.route", "/cart"),
					strKV(attrTracewayDebugIds, debugIds), strKV(attrSessionId, "{0195f2a0-0000-7000-8000-00000000beef}")},
				Events: []*tracepb.Span_Event{{Name: "exception", TimeUnixNano: now + 500_000, Attributes: []*commonpb.KeyValue{
					strKV("exception.type", "TypeError"), strKV("exception.message", "x is undefined"),
					strKV("exception.stacktrace", "TypeError: x is undefined\n    at t (https://cdn.example.com/assets/app-4f2a.js:1:11)"),
					strKV(attrTracewayDebugIds, debugIds),
				}}},
			}},
		}},
	}}}

	result := convertTraces(context.Background(), tokenProject(projectId), projectId, req)
	if len(result.Exceptions) != 1 || len(result.Endpoints) != 1 || len(result.Spans) != 1 {
		t.Fatalf("got %d exceptions, %d endpoints, %d spans", len(result.Exceptions), len(result.Endpoints), len(result.Spans))
	}
	if !strings.Contains(result.Exceptions[0].StackTrace, "original.js") {
		t.Errorf("debug ids must still symbolicate: %q", result.Exceptions[0].StackTrace)
	}
	for name, attrs := range map[string]map[string]string{"exception": result.Exceptions[0].Attributes, "endpoint": result.Endpoints[0].Attributes} {
		if _, ok := attrs[attrTracewayDebugIds]; ok {
			t.Errorf("%s attributes carry the debug ids: %v", name, attrs)
		}
		if attrs[attrSessionId] != canonicalTestSession {
			t.Errorf("%s session.id = %q", name, attrs[attrSessionId])
		}
	}

	row, err := shared.NewOtelSpanRow(result.Spans[0], shared.OtelSpanGroups{})
	if err != nil {
		t.Fatal(err)
	}
	values, err := row.TextValues()
	if err != nil {
		t.Fatal(err)
	}
	var spanAttributes map[string]string
	if err := json.Unmarshal([]byte(values[0].(string)), &spanAttributes); err != nil {
		t.Fatal(err)
	}
	if _, ok := spanAttributes[attrTracewayDebugIds]; ok || spanAttributes[attrSessionId] != canonicalTestSession {
		t.Errorf("span_attributes = %v", spanAttributes)
	}
	for i, column := range []string{"resource", "scope", "events", "links"} {
		text := values[i+1].(string)
		if strings.Contains(text, attrTracewayDebugIds) {
			t.Errorf("readable %s column carries the debug ids: %s", column, text)
		}
		if strings.Contains(strings.ToLower(text), "0195f2a0") && !strings.Contains(text, canonicalTestSession) {
			t.Errorf("readable %s column keeps a non-canonical session id: %s", column, text)
		}
	}
	if payload := values[5].([]byte); bytes.Count(payload, []byte(attrTracewayDebugIds)) != 3 ||
		!bytes.Contains(payload, []byte("0195F2A0-0000-7000-8000-00000000BEEF")) {
		t.Errorf("the lossless payload must keep the attributes exactly as sent")
	}
}
