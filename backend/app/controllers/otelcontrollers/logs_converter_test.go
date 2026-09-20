package otelcontrollers

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"
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

	records := convertLogs(tokenProject(projectId), context.Background(), projectId, req)
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

func TestConvertLogs_ScopeNameDetectsJs(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000ab")
	setFakeStore(t, nil)

	rawStack := "Error: boom\n    at t (https://cdn.example.com/assets/minified.js:1:11)"
	req := logsRequest("", "@opentelemetry/winston-transport", strKV("exception.stacktrace", rawStack))

	records := convertLogs(nil, context.Background(), projectId, req)
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

	records := convertLogs(nil, context.Background(), testProjectId, req)
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

	records := convertLogs(nil, context.Background(), projectId, req)
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
