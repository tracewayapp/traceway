package otelcontrollers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/controllers/clientcontrollers"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/services"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

const testDebugId = "c0ffee00-1234-4abc-8def-0123456789ab"

func browserProject(projectId uuid.UUID) *models.Project {
	p := tokenProject(projectId)
	p.Framework = "react"
	return p
}

func eventLogs(resource []*commonpb.KeyValue, records ...*logspb.LogRecord) *collogspb.ExportLogsServiceRequest {
	return &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			Resource: &resourcepb.Resource{Attributes: resource},
			ScopeLogs: []*logspb.ScopeLogs{{
				Scope:      &commonpb.InstrumentationScope{Name: "@tracewayapp/frontend"},
				LogRecords: records,
			}},
		}},
	}
}

func browserResource(extra ...*commonpb.KeyValue) []*commonpb.KeyValue {
	return append([]*commonpb.KeyValue{
		strKV("service.name", "shop-web"),
		strKV("service.version", "2.3.1"),
		strKV("telemetry.sdk.language", "webjs"),
		strKV("browser.name", "Chrome"),
		strKV("host.name", "ignored"),
	}, extra...)
}

func eventRecord(name string, at time.Time, attrs ...*commonpb.KeyValue) *logspb.LogRecord {
	return &logspb.LogRecord{EventName: name, TimeUnixNano: uint64(at.UnixNano()), Attributes: attrs}
}

func TestConvertLogs_ExceptionEventBecomesIssue(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000001a1")
	setFakeStore(t, map[string][]byte{
		services.SourceMapStorageKey(projectId, services.DebugIdMapName(testDebugId)): []byte(testSourceMap),
	})
	exceptionId := uuid.MustParse("0195f2a0-0000-7000-8000-00000000abcd")
	sessionId := uuid.MustParse("0195f2a0-0000-7000-8000-00000000beef")
	at := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	req := eventLogs(browserResource(), eventRecord(eventException, at,
		strKV("exception.type", "TypeError"),
		strKV("exception.message", "x is undefined"),
		strKV("exception.stacktrace", "TypeError: x is undefined\n    at t (https://cdn.example.com/assets/app-4f2a.js:1:11)"),
		strKV(attrTracewayExceptionId, exceptionId.String()),
		strKV(attrTracewayDebugIds, `{"app-4f2a.js":"`+testDebugId+`"}`),
		strKV(attrSessionId, sessionId.String()),
	))

	result := convertLogs(context.Background(), browserProject(projectId), projectId, "", req)
	if len(result.Exceptions) != 1 || len(result.Records) != 1 {
		t.Fatalf("got %d exceptions and %d records, want 1 and 1", len(result.Exceptions), len(result.Records))
	}
	exc := result.Exceptions[0]
	if exc.Id != exceptionId {
		t.Errorf("id = %s, want the client id %s", exc.Id, exceptionId)
	}
	if !strings.HasPrefix(exc.StackTrace, "TypeError: x is undefined\n") || !strings.Contains(exc.StackTrace, "original.js") {
		t.Errorf("stack trace not symbolicated through the debug id: %q", exc.StackTrace)
	}
	if exc.ExceptionHash != clientcontrollers.ComputeExceptionHash(exc.StackTrace, false) {
		t.Errorf("hash does not match the stored stack trace")
	}
	if exc.SessionId == nil || *exc.SessionId != sessionId {
		t.Errorf("session id = %v, want %s", exc.SessionId, sessionId)
	}
	if exc.AppVersion != "2.3.1" || exc.ServerName != "shop-web" || exc.IsMessage {
		t.Errorf("unexpected row metadata: %+v", exc)
	}
	if !exc.RecordedAt.Equal(at) {
		t.Errorf("recordedAt = %s, want %s", exc.RecordedAt, at)
	}
	if exc.Attributes["browser.name"] != "Chrome" {
		t.Errorf("client resource attribute missing: %v", exc.Attributes)
	}
	for _, key := range []string{"host.name", "exception.stacktrace", attrTracewayDebugIds, attrTracewayExceptionId, attrEventName} {
		if _, ok := exc.Attributes[key]; ok {
			t.Errorf("attribute %q must not be on the issue: %v", key, exc.Attributes)
		}
	}
	record := result.Records[0]
	if !strings.Contains(record.LogAttributes["exception.stacktrace"], "original.js") {
		t.Errorf("stored exception log must use the symbolicated stack: %q", record.LogAttributes["exception.stacktrace"])
	}
	if record.LogAttributes[attrEventName] != eventException {
		t.Errorf("event.name not stamped on the stored record: %v", record.LogAttributes)
	}
	if _, ok := record.LogAttributes[attrTracewayDebugIds]; ok {
		t.Errorf("debug ids must not be stored on the log record")
	}
}

func TestConvertLogs_ExceptionIdFallbackIsDeterministic(t *testing.T) {
	setFakeStore(t, nil)
	at := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	req := eventLogs(browserResource(), eventRecord(eventException, at,
		strKV("exception.type", "Error"),
		strKV("exception.message", "boom"),
		strKV(attrTracewayExceptionId, "not-a-uuid"),
	))

	first := convertLogs(context.Background(), nil, testProjectId, "", req).Exceptions
	second := convertLogs(context.Background(), nil, testProjectId, "", req).Exceptions
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected one exception per conversion")
	}
	if first[0].Id == uuid.Nil || first[0].Id != second[0].Id {
		t.Errorf("a retried record must map to the same id: %s vs %s", first[0].Id, second[0].Id)
	}
}

func TestConvertLogs_MessageEventBecomesMessageIssue(t *testing.T) {
	setFakeStore(t, nil)
	req := eventLogs(browserResource(), &logspb.LogRecord{
		EventName:    eventMessage,
		TimeUnixNano: uint64(time.Now().UnixNano()),
		Body:         &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "checkout failed for order 123456"}},
	}, eventRecord(eventMessage, time.Now()))

	result := convertLogs(context.Background(), browserProject(testProjectId), testProjectId, "", req)
	if len(result.Exceptions) != 1 {
		t.Fatalf("got %d issues, want 1 (an empty message is skipped)", len(result.Exceptions))
	}
	msg := result.Exceptions[0]
	if !msg.IsMessage || msg.StackTrace != "checkout failed for order 123456" {
		t.Errorf("unexpected message row: %+v", msg)
	}
	if msg.ExceptionHash != clientcontrollers.ComputeExceptionHash(msg.StackTrace, true) {
		t.Errorf("messages must hash without normalization")
	}
}

func TestConvertLogs_BackendProjectKeepsResourceOffIssue(t *testing.T) {
	setFakeStore(t, nil)
	req := eventLogs(browserResource(), eventRecord(eventException, time.Now(), strKV("exception.message", "boom")))
	result := convertLogs(context.Background(), tokenProject(testProjectId), testProjectId, "", req)
	if len(result.Exceptions) != 1 {
		t.Fatalf("expected one exception")
	}
	if _, ok := result.Exceptions[0].Attributes["browser.name"]; ok {
		t.Errorf("resource attributes are copied for client-side projects only")
	}
}

func TestConvertLogs_SessionLifecycleCollapses(t *testing.T) {
	setFakeStore(t, nil)
	sessionId := uuid.MustParse("0195f2a0-0000-7000-8000-00000000cafe")
	started := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	startedMs := intKV(attrSessionStarted, started.UnixMilli())
	sid := strKV(attrSessionId, sessionId.String())

	req := eventLogs(browserResource(),
		eventRecord(eventSessionStart, started, sid, startedMs, strKV("url", "/landing"), strKV("client.ip", "6.6.6.6")),
		eventRecord(eventSessionUpdate, started.Add(time.Minute), sid, startedMs, strKV("url", "/landing"), strKV("user.id", "u-42")),
		eventRecord(eventSessionEnd, started.Add(10*time.Minute), sid, startedMs, strKV("url", "/landing"), strKV("user.id", "u-42")),
	)

	result := convertLogs(context.Background(), browserProject(testProjectId), testProjectId, "203.0.113.9", req)
	if len(result.Sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(result.Sessions))
	}
	s := result.Sessions[0]
	if s.Id != sessionId || s.ProjectId != testProjectId || !s.StartedAt.Equal(started) {
		t.Errorf("unexpected identity: %+v", s)
	}
	if s.EndedAt == nil || !s.EndedAt.Equal(started.Add(10*time.Minute)) || s.Duration != (10*time.Minute).Nanoseconds() {
		t.Errorf("end not recorded: endedAt=%v duration=%d", s.EndedAt, s.Duration)
	}
	if s.ClientIP != "203.0.113.9" || s.Attributes["client.ip"] != "203.0.113.9" {
		t.Errorf("client ip must come from the request, got %q / %q", s.ClientIP, s.Attributes["client.ip"])
	}
	if s.Attributes["user.id"] != "u-42" || s.Attributes["browser.name"] != "Chrome" || s.AppVersion != "2.3.1" {
		t.Errorf("latest snapshot and resource not applied: %v", s.Attributes)
	}
	for _, key := range []string{attrSessionId, attrSessionStarted, attrEventName} {
		if _, ok := s.Attributes[key]; ok {
			t.Errorf("attribute %q must not be stored on the session", key)
		}
	}
}

func TestConvertLogs_SessionEventWithoutStartIsDropped(t *testing.T) {
	setFakeStore(t, nil)
	sid := strKV(attrSessionId, "0195f2a0-0000-7000-8000-00000000cafe")
	req := eventLogs(browserResource(),
		eventRecord(eventSessionEnd, time.Now(), sid),
		eventRecord(eventSessionStart, time.Now(), strKV(attrSessionId, "not-a-uuid")),
	)
	result := convertLogs(context.Background(), browserProject(testProjectId), testProjectId, "", req)
	if len(result.Sessions) != 0 {
		t.Fatalf("got %d sessions, want none: an end without its start and a non-UUID id are both unusable", len(result.Sessions))
	}
	if len(result.Records) != 2 {
		t.Errorf("the records themselves are still stored")
	}
}

func TestConvertTraces_BrowserExceptionEventCarriesClientContext(t *testing.T) {
	setFakeStore(t, nil)
	exceptionId := uuid.MustParse("0195f2a0-0000-7000-8000-0000000000e1")
	sessionId := uuid.MustParse("0195f2a0-0000-7000-8000-0000000000e2")
	at := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	span := &tracepb.Span{
		TraceId:           []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
		Name:              "GET",
		Kind:              tracepb.Span_SPAN_KIND_CLIENT,
		StartTimeUnixNano: uint64(at.UnixNano()),
		EndTimeUnixNano:   uint64(at.Add(time.Second).UnixNano()),
		Attributes:        []*commonpb.KeyValue{strKV(attrSessionId, sessionId.String())},
		Events: []*tracepb.Span_Event{{
			Name:         "exception",
			TimeUnixNano: uint64(at.UnixNano()),
			Attributes: []*commonpb.KeyValue{
				strKV("exception.type", "Error"),
				strKV("exception.message", "boom"),
				strKV(attrTracewayExceptionId, exceptionId.String()),
			},
		}},
	}
	req := &coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource:   &resourcepb.Resource{Attributes: browserResource()},
		ScopeSpans: []*tracepb.ScopeSpans{{Scope: &commonpb.InstrumentationScope{Name: "@opentelemetry/browser-instrumentation"}, Spans: []*tracepb.Span{span}}},
	}}}

	exceptions := convertTraces(context.Background(), browserProject(testProjectId), testProjectId, req).Exceptions
	if len(exceptions) != 1 {
		t.Fatalf("expected one exception, got %d", len(exceptions))
	}
	exc := exceptions[0]
	if exc.Id != exceptionId {
		t.Errorf("id = %s, want the client id", exc.Id)
	}
	if exc.SessionId == nil || *exc.SessionId != sessionId {
		t.Errorf("session id = %v, want %s", exc.SessionId, sessionId)
	}
	if exc.Attributes["browser.name"] != "Chrome" || exc.Attributes["telemetry.sdk.language"] != "webjs" {
		t.Errorf("client attributes missing: %v", exc.Attributes)
	}

	backend := convertTraces(context.Background(), tokenProject(testProjectId), testProjectId, req).Exceptions
	if _, ok := backend[0].Attributes["browser.name"]; ok {
		t.Errorf("backend projects keep their exception attributes unchanged")
	}
}

func TestConvertLogs_FlutterExceptionKeepsDeviceContextAndTrace(t *testing.T) {
	setFakeStore(t, nil)
	project := tokenProject(testProjectId)
	project.Framework = "flutter"
	exceptionId := uuid.MustParse("0195f2a0-0000-7000-8000-00000000f1a7")
	sessionId := uuid.MustParse("0195f2a0-0000-7000-8000-00000000f1a8")
	trace := "StateError: Bad state: boom\nmain.<anonymous closure>\n    package:app/main.dart:42:7"

	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
				strKV("service.name", "shop-app"),
				strKV("service.version", "1.4.0"),
				strKV("telemetry.sdk.language", "dart"),
				strKV("device.model", "Pixel 8"),
				strKV("os.name", "android"),
				strKV("host.name", "ignored"),
			}},
			ScopeLogs: []*logspb.ScopeLogs{{
				Scope: &commonpb.InstrumentationScope{Name: "traceway-flutter"},
				LogRecords: []*logspb.LogRecord{eventRecord(eventException, time.Now(),
					strKV("exception.type", "StateError"),
					strKV("exception.message", "Bad state: boom"),
					strKV("exception.stacktrace", trace),
					strKV(attrTracewayExceptionId, exceptionId.String()),
					strKV(attrSessionId, sessionId.String()),
				)},
			}},
		}},
	}
	result := convertLogs(context.Background(), project, testProjectId, "", req)
	if len(result.Exceptions) != 1 {
		t.Fatalf("expected one exception, got %d", len(result.Exceptions))
	}
	issue := result.Exceptions[0]
	if issue.Id != exceptionId {
		t.Errorf("the SDK's exception id must become the issue id so a replay clip can name it, got %s", issue.Id)
	}
	if issue.SessionId == nil || *issue.SessionId != sessionId {
		t.Errorf("session id not linked: %v", issue.SessionId)
	}
	if issue.StackTrace != trace {
		t.Errorf("a readable Dart trace must be stored as sent, got %q", issue.StackTrace)
	}
	if issue.Attributes["device.model"] != "Pixel 8" || issue.Attributes["os.name"] != "android" {
		t.Errorf("device attributes of the resource belong on a Flutter issue: %v", issue.Attributes)
	}
	if _, ok := issue.Attributes["host.name"]; ok {
		t.Errorf("only client resource attributes are copied")
	}
	if issue.AppVersion != "1.4.0" || issue.ServerName != "shop-app" {
		t.Errorf("version/service not carried: %q %q", issue.AppVersion, issue.ServerName)
	}
}
