package otelcontrollers

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/services"
	"github.com/tracewayapp/traceway/backend/v2/app/storage"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

const testSourceMap = `{"version":3,"names":["abcd"],"sources":["tests/fixtures/simple/original.js"],"sourcesContent":["// ./node_modules/.bin/terser -c -m --module tests/fixtures/simple/original.js --source-map includeSources -o tests/fixtures/simple/minified.js\nfunction abcd() {}\nexport default abcd;\n"],"mappings":"AACA,SAASA,oBACMA"}`

type fakeStore struct{ files map[string][]byte }

func (s *fakeStore) Read(_ context.Context, key string) ([]byte, error) {
	if b, ok := s.files[key]; ok {
		return b, nil
	}
	return nil, storage.ErrNotFound
}

func (s *fakeStore) Write(context.Context, string, []byte) error { return nil }

func (s *fakeStore) Delete(context.Context, string) error { return nil }

func setFakeStore(t *testing.T, files map[string][]byte) {
	t.Helper()
	prev := storage.Store
	storage.Store = &fakeStore{files: files}
	t.Cleanup(func() { storage.Store = prev })
}

func logsRequest(language, scopeName string, attrs ...*commonpb.KeyValue) *collogspb.ExportLogsServiceRequest {
	var resAttrs []*commonpb.KeyValue
	if language != "" {
		resAttrs = append(resAttrs, strKV("telemetry.sdk.language", language))
	}
	return &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			Resource: &resourcepb.Resource{Attributes: resAttrs},
			ScopeLogs: []*logspb.ScopeLogs{{
				Scope:      &commonpb.InstrumentationScope{Name: scopeName},
				LogRecords: []*logspb.LogRecord{{Attributes: attrs}},
			}},
		}},
	}
}

func TestConvertLogs_SymbolicatesExceptionStacktrace(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	setFakeStore(t, map[string][]byte{
		services.SourceMapStorageKey(projectId, "minified.js.map"): []byte(testSourceMap),
	})

	rawStack := "Error: boom\n    at t (https://cdn.example.com/assets/minified.js:1:11)"
	req := logsRequest("webjs", "@opentelemetry/winston-transport",
		strKV("exception.type", "Error"),
		strKV("exception.stacktrace", rawStack),
	)

	records := convertLogs(context.Background(), tokenProject(projectId), projectId, "", req).Records
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	got := records[0].LogAttributes["exception.stacktrace"]
	if !strings.Contains(got, "original.js") {
		t.Errorf("expected stack trace resolved via source map, got %q", got)
	}
	if got := records[0].LogAttributes["exception.type"]; got != "Error" {
		t.Errorf("expected other attributes untouched, got exception.type=%q", got)
	}
}

// The iOS SDK sends an error's frames as binary-image offsets; with the build's dSYM uploaded, the stored record
// and the issue carry function, file and line, as on the native report protocol.
func TestConvertLogs_SymbolicatesIosStacktrace(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000ac")
	dsym, err := os.ReadFile(filepath.Join("..", "..", "symbolicator", "ios", "fixtures", "sample", "sample.dsym"))
	if err != nil {
		t.Fatal(err)
	}
	trace, err := os.ReadFile(filepath.Join("..", "..", "symbolicator", "ios", "fixtures", "sample", "trace.txt"))
	if err != nil {
		t.Fatal(err)
	}
	setFakeStore(t, map[string][]byte{
		services.IOSSymbolsKey(projectId, "2dd71042118432be8f92dd4e3d3fe24a"): dsym,
	})
	req := logsRequest("swift", "traceway-ios",
		strKV("event.name", "exception"),
		strKV("exception.type", "SampleError"),
		strKV("exception.message", "boom"),
		strKV("exception.stacktrace", string(trace)),
	)
	project := tokenProject(projectId)
	project.Framework = "ios"
	converted := convertLogs(context.Background(), project, projectId, "", req)
	if len(converted.Records) != 1 || len(converted.Exceptions) != 1 {
		t.Fatalf("expected one record and one issue, got %d and %d", len(converted.Records), len(converted.Exceptions))
	}
	for _, got := range []string{converted.Records[0].LogAttributes["exception.stacktrace"], converted.Exceptions[0].StackTrace} {
		if !strings.Contains(got, "leaf (sample.c:7:18)") || !strings.Contains(got, "main (sample.c:18)") {
			t.Errorf("expected the frames resolved through the dSYM, got %q", got)
		}
		if !strings.HasPrefix(got, "SampleError: boom") {
			t.Errorf("expected the error line first, got %q", got)
		}
	}
}

// The Android SDK sends an exception as a log event; with the build's R8 mapping uploaded, the issue carries the
// original class, method and file names, as on the span path.
func TestConvertLogs_RetracesAndroidStacktrace(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000ad")
	const proguardUuid = "6A8CB813-45F6-3652-AD33-778FD1EAB196"
	setFakeStore(t, map[string][]byte{
		services.AndroidMappingKey(projectId, proguardUuid): androidConvFixture(t, "r8", "mapping.txt"),
	})
	req := logsRequest("java", "traceway-android",
		strKV("event.name", "exception"),
		strKV("exception.type", "a.b"),
		strKV("exception.message", "card declined for $30.59"),
		strKV("exception.stacktrace", string(androidConvFixture(t, "r8", "obfuscated.txt"))),
	)
	req.ResourceLogs[0].Resource.Attributes = append(req.ResourceLogs[0].Resource.Attributes,
		strKV("app.debug.proguard_uuid", proguardUuid))
	project := tokenProject(projectId)
	project.Framework = "android"
	converted := convertLogs(context.Background(), project, projectId, "", req)
	if len(converted.Exceptions) != 1 {
		t.Fatalf("expected one issue, got %d", len(converted.Exceptions))
	}
	got := converted.Exceptions[0].StackTrace
	if strings.Contains(got, "a.a.a(") {
		t.Errorf("trace still obfuscated:\n%s", got)
	}
	for _, want := range []string{"com.example.demo.Checkout.chargeCard", "Checkout.java", "PaymentDeclinedException"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in the retraced issue:\n%s", want, got)
		}
	}
	// TW-11: the stored log record (Logs page, session timeline, trace correlation) is retraced too, as on iOS.
	if len(converted.Records) != 1 {
		t.Fatalf("expected one record, got %d", len(converted.Records))
	}
	record := converted.Records[0].LogAttributes
	if st := record["exception.stacktrace"]; strings.Contains(st, "a.a.a(") || !strings.Contains(st, "com.example.demo.Checkout.chargeCard") {
		t.Errorf("record trace not retraced:\n%s", st)
	}
	if typ := record["exception.type"]; !strings.HasSuffix(typ, "PaymentDeclinedException") {
		t.Errorf("record exception.type = %q, want the original class", typ)
	}
	if typ := converted.Exceptions[0].Attributes["exception.type"]; !strings.HasSuffix(typ, "PaymentDeclinedException") {
		t.Errorf("issue exception.type attribute = %q, want the original class", typ)
	}
	obfuscated := string(androidConvFixture(t, "r8", "obfuscated.txt"))
	once := otelSymbolicateAndroid(project, projectId, context.Background(),
		formatExceptionStackTrace("a.b", "card declined for $30.59", obfuscated), "java", proguardUuid)
	if got != once {
		t.Errorf("the issue must be retraced once, from what the SDK sent:\n got %q\nwant %q", got, once)
	}
}

func TestConvertLogs_ScopeNameDetectsJs(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000ab")
	setFakeStore(t, nil)

	rawStack := "Error: boom\n    at t (https://cdn.example.com/assets/minified.js:1:11)"
	req := logsRequest("", "@opentelemetry/winston-transport", strKV("exception.stacktrace", rawStack))

	records := convertLogs(context.Background(), nil, projectId, "", req).Records
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	want := "Error: boom\nt()\n    https://cdn.example.com/assets/minified.js:1:11"
	if got := records[0].LogAttributes["exception.stacktrace"]; got != want {
		t.Errorf("expected canonicalized stack trace stored:\n got %q\nwant %q", got, want)
	}
}

func TestConvertLogs_NonJsStacktraceUntouched(t *testing.T) {
	setFakeStore(t, nil)

	rawStack := "java.lang.RuntimeException: boom\n\tat com.example.Foo.bar(Foo.java:10)"
	req := logsRequest("java", "io.opentelemetry.tomcat-7.0", strKV("exception.stacktrace", rawStack))

	records := convertLogs(context.Background(), nil, testProjectId, "", req).Records
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if got := records[0].LogAttributes["exception.stacktrace"]; got != rawStack {
		t.Errorf("expected non-JS stack trace untouched, got %q", got)
	}
}

func decodeLogIDs(t *testing.T, encoding string, traceID, spanID []byte) *logspb.LogRecord {
	t.Helper()
	var body []byte
	contentType := "application/json"
	if encoding == "protobuf" {
		request := &collogspb.ExportLogsServiceRequest{ResourceLogs: []*logspb.ResourceLogs{{ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{{TraceId: traceID, SpanId: spanID}}}}}}}
		var err error
		body, err = proto.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		contentType = "application/x-protobuf"
	} else {
		body = []byte(fmt.Sprintf(`{"resourceLogs":[{"scopeLogs":[{"logRecords":[{"traceId":%q,"spanId":%q}]}]}]}`, hex.EncodeToString(traceID), hex.EncodeToString(spanID)))
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/otel/v1/logs", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	req, size, err := decodeLogsRequest(c)
	if err != nil {
		t.Fatal(err)
	}
	if size != len(body) {
		t.Fatalf("body size = %d, want %d", size, len(body))
	}
	return req.ResourceLogs[0].ScopeLogs[0].LogRecords[0]
}

func TestToLogRecord_IDEncoding(t *testing.T) {
	const wireTraceHex = "7b873c7bbf35739e79e1f7b9736739f7"
	const wireSpanHex = "7dfd3877775ae1bd"
	traceID, _ := hex.DecodeString(wireTraceHex)
	spanID, _ := hex.DecodeString(wireSpanHex)

	for _, encoding := range []string{"protobuf", "json"} {
		for _, tt := range []struct {
			name                string
			trace, span         []byte
			wantTrace, wantSpan string
		}{
			{"valid", traceID, spanID, wireTraceHex, wireSpanHex},
			{"missing", nil, nil, "", ""},
			{"zero", make([]byte, 16), make([]byte, 8), "", ""},
			{"short", []byte{1}, []byte{1}, "", ""},
			{"long", bytes.Repeat([]byte{1}, 24), bytes.Repeat([]byte{1}, 12), "", ""},
			{"trace only", traceID, nil, wireTraceHex, ""},
			{"span only", nil, spanID, "", wireSpanHex},
		} {
			t.Run(encoding+"/"+tt.name, func(t *testing.T) {
				lr := decodeLogIDs(t, encoding, tt.trace, tt.span)
				if !bytes.Equal(lr.TraceId, tt.trace) || !bytes.Equal(lr.SpanId, tt.span) {
					t.Fatalf("decoder changed ID bytes: %v", lr)
				}
				rec := toLogRecord(testProjectId, lr, "svc", "", nil, "", "scope", "", nil)
				if rec.TraceId != tt.wantTrace || rec.SpanId != tt.wantSpan {
					t.Fatalf("IDs = %q/%q, want %q/%q", rec.TraceId, rec.SpanId, tt.wantTrace, tt.wantSpan)
				}
			})
		}
	}
}

func TestConvertLogs_NoTokenCanonicalizesWithoutResolving(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000ae")
	setFakeStore(t, map[string][]byte{
		services.SourceMapStorageKey(projectId, "minified.js.map"): []byte(testSourceMap),
	})

	rawStack := "Error: boom\n    at t (https://cdn.example.com/assets/minified.js:1:11)"
	req := logsRequest("webjs", "@opentelemetry/winston-transport",
		strKV("exception.type", "Error"),
		strKV("exception.stacktrace", rawStack),
	)

	records := convertLogs(context.Background(), nil, projectId, "", req).Records
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	got := records[0].LogAttributes["exception.stacktrace"]
	want := "Error: boom\nt()\n    https://cdn.example.com/assets/minified.js:1:11"
	if got != want {
		t.Errorf("expected canonicalized-but-unresolved stack without a token, got %q", got)
	}
	if _, ok := records[0].LogAttributes["exception.stacktrace.original"]; ok {
		t.Error("exception.stacktrace.original must not be added")
	}
}

func TestSessionPreservesTracewaySDKVersion(t *testing.T) {
	sessionID := uuid.New().String()
	for _, scope := range []string{"@tracewayapp/frontend", "traceway-flutter", "other-sdk"} {
		t.Run(scope, func(t *testing.T) {
			record := models.LogRecord{ScopeName: scope, ScopeVersion: "2.3.1"}
			log := &logspb.LogRecord{
				TimeUnixNano: uint64(time.Now().UnixNano()),
				Attributes: []*commonpb.KeyValue{{
					Key:   "session.id",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: sessionID}},
				}},
			}
			session, ok := sessionFromLog(eventSessionStart, log, nil, record, "", time.Now())
			if !ok {
				t.Fatal("session was not converted")
			}
			expected := ""
			if scope != "other-sdk" {
				expected = "2.3.1"
			}
			if got := session.Attributes["traceway.sdk.version"]; got != expected {
				t.Fatalf("SDK version = %q, want %q", got, expected)
			}
		})
	}
}

// TW-12: a session's start comes from the device as it is; one no session could have (a unit slip, such as
// microseconds sent as milliseconds, or seconds) is ignored like a missing one instead of being stored past what
// ClickHouse's DateTime holds.
func TestSessionStartedAtIsBoundedByTheEventTime(t *testing.T) {
	event := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	started := event.Add(-10 * time.Minute)
	for name, attrs := range map[string][]*commonpb.KeyValue{
		"ms":     {intKV(attrSessionStarted, started.UnixMilli())},
		"string": {strKV(attrSessionStarted, fmt.Sprint(started.UnixMilli()))},
		"rfc":    {strKV(attrSessionStarted, started.Format(time.RFC3339Nano))},
	} {
		if got, ok := sessionStartedAt(attrs, event); !ok || !got.Equal(started) {
			t.Errorf("%s: got %v %v, want %v", name, got, ok, started)
		}
	}
	for name, attrs := range map[string][]*commonpb.KeyValue{
		"microseconds": {intKV(attrSessionStarted, started.UnixMicro())},
		"seconds":      {intKV(attrSessionStarted, started.Unix())},
		"far future":   {strKV(attrSessionStarted, "2199-01-01T00:00:00Z")},
		"after event":  {intKV(attrSessionStarted, event.Add(time.Hour).UnixMilli())},
		"days before":  {intKV(attrSessionStarted, event.Add(-72*time.Hour).UnixMilli())},
	} {
		if got, ok := sessionStartedAt(attrs, event); ok {
			t.Errorf("%s: accepted %v", name, got)
		}
	}
}
