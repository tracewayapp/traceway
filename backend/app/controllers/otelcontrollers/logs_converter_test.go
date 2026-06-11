package otelcontrollers

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/storage"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
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

// Pins the protojson base64-of-hex round-trip — see logs_converter.go.
func TestToLogRecord_IDEncoding(t *testing.T) {
	const wireTraceHex = "7b873c7bbf35739e79e1f7b9736739f7"
	const wireSpanHex = "7dfd3877775ae1bd"

	// protojson decodes bytes-typed fields as base64; hex chars are valid base64.
	jsonTrace, err := base64.StdEncoding.DecodeString(wireTraceHex)
	if err != nil || len(jsonTrace) != 24 {
		t.Fatalf("seed: jsonTrace len=%d err=%v", len(jsonTrace), err)
	}
	jsonSpan, err := base64.StdEncoding.DecodeString(wireSpanHex)
	if err != nil || len(jsonSpan) != 12 {
		t.Fatalf("seed: jsonSpan len=%d err=%v", len(jsonSpan), err)
	}
	binTrace, _ := hex.DecodeString(wireTraceHex)
	binSpan, _ := hex.DecodeString(wireSpanHex)

	tests := []struct {
		name       string
		traceBytes []byte
		spanBytes  []byte
		wantTrace  string
		wantSpan   string
	}{
		{"binary OTLP", binTrace, binSpan, wireTraceHex, wireSpanHex},
		{"JSON OTLP (protojson base64 roundtrip)", jsonTrace, jsonSpan, wireTraceHex, wireSpanHex},
		{"missing trace context", nil, nil, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := toLogRecord(
				testProjectId,
				&logspb.LogRecord{TraceId: tt.traceBytes, SpanId: tt.spanBytes},
				"svc", "", nil, "", "scope", "", nil,
			)
			if rec.TraceId != tt.wantTrace {
				t.Errorf("TraceId = %q (len %d), want %q", rec.TraceId, len(rec.TraceId), tt.wantTrace)
			}
			if rec.SpanId != tt.wantSpan {
				t.Errorf("SpanId = %q (len %d), want %q", rec.SpanId, len(rec.SpanId), tt.wantSpan)
			}
		})
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
