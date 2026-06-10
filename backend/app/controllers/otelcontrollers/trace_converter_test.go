package otelcontrollers

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/services"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

var update = flag.Bool("update", false, "update golden files")

var testProjectId = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// --- Snapshot types (stable, deterministic output) ---

type snapshotEndpoint struct {
	Endpoint   string `json:"endpoint"`
	StatusCode int16  `json:"statusCode"`
	ServerName string `json:"serverName"`
	AppVersion string `json:"appVersion"`
}

type snapshotSpan struct {
	Name           string `json:"name"`
	LinkedToParent bool   `json:"linkedToParent"`
}

type snapshotException struct {
	StackTrace string `json:"stackTrace"`
	TraceType  string `json:"traceType"`
}

type snapshotAiTrace struct {
	TraceName       string  `json:"traceName"`
	Model           string  `json:"model"`
	Provider        string  `json:"provider"`
	Operation       string  `json:"operation"`
	InputTokens     int64   `json:"inputTokens"`
	OutputTokens    int64   `json:"outputTokens"`
	TotalTokens     int64   `json:"totalTokens"`
	CachedTokens    int64   `json:"cachedTokens"`
	ReasoningTokens int64   `json:"reasoningTokens"`
	InputCost       float64 `json:"inputCost"`
	OutputCost      float64 `json:"outputCost"`
	TotalCost       float64 `json:"totalCost"`
	FinishReason    string  `json:"finishReason"`
	StatusCode      uint8   `json:"statusCode"`
}

type snapshotResult struct {
	EndpointCount     int                 `json:"endpointCount"`
	Endpoints         []snapshotEndpoint  `json:"endpoints"`
	TaskCount         int                 `json:"taskCount"`
	SpanCount         int                 `json:"spanCount"`
	Spans             []snapshotSpan      `json:"spans"`
	ExceptionCount    int                 `json:"exceptionCount"`
	Exceptions        []snapshotException `json:"exceptions"`
	AiTraceCount      int                 `json:"aiTraceCount"`
	AiTraces          []snapshotAiTrace   `json:"aiTraces"`
	ConversationCount int                 `json:"conversationCount"`
	AllSpansLinked    bool                `json:"allSpansLinked"`
}

// --- Snapshot tests ---

func TestConvertTraces_Snapshot(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
	}{
		{"openrouter_ai_trace", "testdata/openrouter_ai_trace.json"},
		{"node_better_auth", "testdata/node_better_auth.json"},
		{"node_sign_in", "testdata/node_sign_in.json"},
		{"spring_boot_exception", "testdata/spring_boot_exception.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := os.ReadFile(tt.fixture)
			if err != nil {
				t.Fatalf("failed to read fixture %s: %v", tt.fixture, err)
			}

			req := &coltracepb.ExportTraceServiceRequest{}
			if err := protojson.Unmarshal(raw, req); err != nil {
				t.Fatalf("failed to unmarshal fixture %s: %v", tt.fixture, err)
			}

			setFakeStore(t, nil)
			endpoints, _, spans, exceptions, aiTraces, aiConversations := convertTraces(context.Background(), testProjectId, req)

			// Check if all child spans share a trace ID with an endpoint
			endpointIds := map[uuid.UUID]bool{}
			for _, ep := range endpoints {
				endpointIds[ep.Id] = true
			}
			for _, at := range aiTraces {
				endpointIds[at.Id] = true
			}
			allLinked := true
			snapSpans := make([]snapshotSpan, len(spans))
			for i, s := range spans {
				linked := endpointIds[s.TraceId]
				snapSpans[i] = snapshotSpan{Name: s.Name, LinkedToParent: linked}
				if !linked {
					allLinked = false
				}
			}
			sort.Slice(snapSpans, func(i, j int) bool { return snapSpans[i].Name < snapSpans[j].Name })

			snapEndpoints := make([]snapshotEndpoint, len(endpoints))
			for i, ep := range endpoints {
				snapEndpoints[i] = snapshotEndpoint{
					Endpoint:   ep.Endpoint,
					StatusCode: ep.StatusCode,
					ServerName: ep.ServerName,
					AppVersion: ep.AppVersion,
				}
			}

			snapExceptions := make([]snapshotException, len(exceptions))
			for i, ex := range exceptions {
				snapExceptions[i] = snapshotException{
					StackTrace: ex.StackTrace,
					TraceType:  ex.TraceType,
				}
			}

			snapAiTraces := make([]snapshotAiTrace, len(aiTraces))
			for i, at := range aiTraces {
				snapAiTraces[i] = snapshotAiTrace{
					TraceName:       at.TraceName,
					Model:           at.Model,
					Provider:        at.Provider,
					Operation:       at.Operation,
					InputTokens:     at.InputTokens,
					OutputTokens:    at.OutputTokens,
					TotalTokens:     at.TotalTokens,
					CachedTokens:    at.CachedTokens,
					ReasoningTokens: at.ReasoningTokens,
					InputCost:       at.InputCost,
					OutputCost:      at.OutputCost,
					TotalCost:       at.TotalCost,
					FinishReason:    at.FinishReason,
					StatusCode:      at.StatusCode,
				}
			}

			result := snapshotResult{
				EndpointCount:     len(endpoints),
				Endpoints:         snapEndpoints,
				TaskCount:         0,
				SpanCount:         len(spans),
				Spans:             snapSpans,
				ExceptionCount:    len(exceptions),
				Exceptions:        snapExceptions,
				AiTraceCount:      len(aiTraces),
				AiTraces:          snapAiTraces,
				ConversationCount: len(aiConversations),
				AllSpansLinked:    allLinked,
			}

			got, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				t.Fatalf("failed to marshal result: %v", err)
			}

			golden := tt.fixture + ".golden.json"
			if *update {
				if err := os.WriteFile(golden, got, 0644); err != nil {
					t.Fatalf("failed to write golden file: %v", err)
				}
				t.Logf("updated golden file %s", golden)
				return
			}

			expected, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("golden file %s missing — run with -update flag to generate: %v", golden, err)
			}

			if string(got) != string(expected) {
				t.Errorf("output differs from golden file %s\n\nGot:\n%s\n\nExpected:\n%s", golden, string(got), string(expected))
			}
		})
	}
}

// --- Unit tests ---

func TestHasHTTPAttributes(t *testing.T) {
	tests := []struct {
		name  string
		attrs []*commonpb.KeyValue
		want  bool
	}{
		{"with http.route", makeAttrs("http.route", "/api/users"), true},
		{"with http.request.method", makeAttrs("http.request.method", "GET"), true},
		{"with http.method", makeAttrs("http.method", "POST"), true},
		{"with url.path", makeAttrs("url.path", "/api"), true},
		{"without http attrs", makeAttrs("db.operation.name", "findOne"), false},
		{"empty", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasHTTPAttributes(tt.attrs); got != tt.want {
				t.Errorf("hasHTTPAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasGenAiAttributes(t *testing.T) {
	tests := []struct {
		name  string
		attrs []*commonpb.KeyValue
		want  bool
	}{
		{"with gen_ai.request.model", makeAttrs("gen_ai.request.model", "gpt-4"), true},
		{"with gen_ai.usage.input_tokens", makeAttrs("gen_ai.usage.input_tokens", "50"), true},
		{"without gen_ai", makeAttrs("http.route", "/api"), false},
		{"empty", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasGenAiAttributes(tt.attrs); got != tt.want {
				t.Errorf("hasGenAiAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHTTPEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		attrs    []*commonpb.KeyValue
		fallback string
		want     string
	}{
		{"method+route", append(makeAttrs("http.request.method", "GET"), makeAttrs("http.route", "/api/users")...), "fallback", "GET /api/users"},
		{"old method+path", append(makeAttrs("http.method", "POST"), makeAttrs("url.path", "/submit")...), "fallback", "POST /submit"},
		{"method only", makeAttrs("http.request.method", "DELETE"), "my-op", "DELETE my-op"},
		{"no attrs", nil, "fallback", "fallback"},
		{"route without leading slash ignored", append(makeAttrs("http.request.method", "GET"), makeAttrs("http.route", "no-slash")...), "fallback", "GET fallback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHTTPEndpoint(tt.attrs, tt.fallback); got != tt.want {
				t.Errorf("getHTTPEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterNonStandardAiAttrs(t *testing.T) {
	input := map[string]string{
		"gen_ai.request.model":       "gpt-4",
		"gen_ai.prompt":              "...",
		"gen_ai.usage.input_tokens":  "50",
		"trace.name":                 "My Agent",
		"user.id":                    "u123",
		"custom.tag":                 "production",
		"gen_ai.request.temperature": "0.7",
	}
	result := filterNonStandardAiAttrs(input)

	if _, ok := result["gen_ai.request.model"]; ok {
		t.Error("should not include gen_ai.request.model")
	}
	if _, ok := result["gen_ai.prompt"]; ok {
		t.Error("should not include gen_ai.prompt")
	}
	if _, ok := result["trace.name"]; ok {
		t.Error("should not include trace.name")
	}
	if v, ok := result["custom.tag"]; !ok || v != "production" {
		t.Error("should include custom.tag")
	}
	if v, ok := result["gen_ai.request.temperature"]; !ok || v != "0.7" {
		t.Error("should include gen_ai.request.temperature")
	}
}

func TestExtractConversation(t *testing.T) {
	t.Run("with prompt and completion", func(t *testing.T) {
		attrs := append(
			makeAttrs("gen_ai.prompt", `{"messages":[{"role":"user","content":"hello"}]}`),
			makeAttrs("gen_ai.completion", `{"choices":[{"message":{"content":"hi"}}]}`)...,
		)
		conv := extractConversation(attrs, testProjectId, uuid.New())
		if conv == nil {
			t.Fatal("expected conversation, got nil")
		}
		if len(conv.Content) == 0 {
			t.Error("expected non-empty content")
		}
	})

	t.Run("without prompt or completion", func(t *testing.T) {
		attrs := makeAttrs("gen_ai.request.model", "gpt-4")
		conv := extractConversation(attrs, testProjectId, uuid.New())
		if conv != nil {
			t.Error("expected nil conversation")
		}
	})
}

func TestTraceIdResolution_CrossScope(t *testing.T) {
	// Simulate: child in scope 1 is processed before parent in scope 2
	rootSpanId := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	childSpanId := []byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
	grandchildSpanId := []byte{0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28}
	traceIdBytes := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}

	now := uint64(1700000000000000000)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						strKV("service.name", "test"),
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						// Scope 1: grandchild arrives FIRST
						Spans: []*tracepb.Span{
							{
								TraceId:           traceIdBytes,
								SpanId:            grandchildSpanId,
								ParentSpanId:      childSpanId,
								Name:              "tcp.connect",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: now,
								EndTimeUnixNano:   now + 1000000,
							},
						},
					},
					{
						// Scope 2: child and root arrive AFTER
						Spans: []*tracepb.Span{
							{
								TraceId:           traceIdBytes,
								SpanId:            childSpanId,
								ParentSpanId:      rootSpanId,
								Name:              "db query",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: now,
								EndTimeUnixNano:   now + 2000000,
							},
							{
								TraceId:      traceIdBytes,
								SpanId:       rootSpanId,
								Name:         "GET /api/test",
								Kind:         tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: now,
								EndTimeUnixNano:   now + 5000000,
								Attributes: []*commonpb.KeyValue{
									strKV("http.request.method", "GET"),
									strKV("http.route", "/api/test"),
								},
							},
						},
					},
				},
			},
		},
	}

	endpoints, _, spans, _, _, _ := convertTraces(context.Background(), testProjectId, req)

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	rootTraceId := endpoints[0].Id
	for _, s := range spans {
		if s.TraceId != rootTraceId {
			t.Errorf("span %q has traceId %s, want %s (root endpoint)", s.Name, s.TraceId, rootTraceId)
		}
	}
}

func TestFormatExceptionStackTrace(t *testing.T) {
	tests := []struct {
		name         string
		excType      string
		excMessage   string
		excStacktrace string
		want         string
	}{
		{
			name:         "go style - no stacktrace",
			excType:      "RuntimeError",
			excMessage:   "something failed",
			excStacktrace: "",
			want:         "RuntimeError: something failed",
		},
		{
			name:         "go style - with stacktrace that doesn't start with type",
			excType:      "RuntimeError",
			excMessage:   "something failed",
			excStacktrace: "goroutine 1 [running]:\nmain.foo()\n\t/app/main.go:10",
			want:         "RuntimeError: something failed\ngoroutine 1 [running]:\nmain.foo()\n\t/app/main.go:10",
		},
		{
			// Java/JVM OTel agents include the full "Type: message\n\tat ..." in
			// exception.stacktrace, so we must not prepend a duplicate header.
			name:         "java style - stacktrace already starts with exception type",
			excType:      "org.springframework.dao.EmptyResultDataAccessException",
			excMessage:   "Incorrect result size: expected 1, actual 0",
			excStacktrace: "org.springframework.dao.EmptyResultDataAccessException: Incorrect result size: expected 1, actual 0\n\tat org.springframework.dao.support.DataAccessUtils.requiredSingleResult(DataAccessUtils.java:90)\n\tat com.example.UserService.getUser(UserService.java:38)",
			want:         "org.springframework.dao.EmptyResultDataAccessException: Incorrect result size: expected 1, actual 0\n\tat org.springframework.dao.support.DataAccessUtils.requiredSingleResult(DataAccessUtils.java:90)\n\tat com.example.UserService.getUser(UserService.java:38)",
		},
		{
			name:         "java style - type only, no message",
			excType:      "java.lang.NullPointerException",
			excMessage:   "",
			excStacktrace: "java.lang.NullPointerException\n\tat com.example.Service.run(Service.java:10)",
			want:         "java.lang.NullPointerException\n\tat com.example.Service.run(Service.java:10)",
		},
		{
			name:         "empty everything",
			excType:      "",
			excMessage:   "",
			excStacktrace: "",
			want:         "unknown exception",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatExceptionStackTrace(tt.excType, tt.excMessage, tt.excStacktrace)
			if got != tt.want {
				t.Errorf("formatExceptionStackTrace() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

// --- Non-root classification tests ---

func TestConvertTraces_ConsumerNonRoot_BecomesTask(t *testing.T) {
	// Worker batch: CONSUMER span with a parent that lives in the producer's
	// batch (not present in our scope), plus a child DB span.
	traceId := []byte{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f}
	producerSpanId := []byte{0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5, 0xA6, 0xA7}
	consumerSpanId := []byte{0xB0, 0xB1, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7}
	childSpanId := []byte{0xC0, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7}
	now := uint64(1_700_000_000_000_000_000)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "worker")}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Spans: []*tracepb.Span{
					{TraceId: traceId, SpanId: consumerSpanId, ParentSpanId: producerSpanId, Name: "process job", Kind: tracepb.Span_SPAN_KIND_CONSUMER, StartTimeUnixNano: now, EndTimeUnixNano: now + 1_000_000},
					{TraceId: traceId, SpanId: childSpanId, ParentSpanId: consumerSpanId, Name: "SELECT users", Kind: tracepb.Span_SPAN_KIND_INTERNAL, StartTimeUnixNano: now, EndTimeUnixNano: now + 500_000, Attributes: []*commonpb.KeyValue{strKV("db.system", "postgresql")}},
				},
			}},
		}},
	}

	endpoints, tasks, spans, _, _, _ := convertTraces(context.Background(), testProjectId, req)

	if len(endpoints) != 0 {
		t.Fatalf("expected 0 endpoints, got %d", len(endpoints))
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].IsRoot {
		t.Errorf("expected task.IsRoot == false, got true")
	}
	wantTaskId := otelSpanIDToUUID(consumerSpanId)
	if tasks[0].Id != wantTaskId {
		t.Errorf("expected task.Id %s (from span_id), got %s", wantTaskId, tasks[0].Id)
	}
	wantDtId := otelTraceIDToUUID(traceId)
	if tasks[0].DistributedTraceId == nil || *tasks[0].DistributedTraceId != wantDtId {
		t.Errorf("expected task.DistributedTraceId %s (from trace_id), got %v", wantDtId, tasks[0].DistributedTraceId)
	}

	if len(spans) != 1 {
		t.Fatalf("expected 1 span row, got %d", len(spans))
	}
	if spans[0].TraceId != wantTaskId {
		t.Errorf("expected child span.TraceId == task.Id %s, got %s", wantTaskId, spans[0].TraceId)
	}
	if spans[0].Attributes["db.system"] != "postgresql" {
		t.Errorf("expected child span attribute db.system 'postgresql', got %q", spans[0].Attributes["db.system"])
	}
}

func TestConvertTraces_ConsoleCommand_BecomesTask(t *testing.T) {
	traceId := []byte{0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f}
	rootSpanId := []byte{0xD0, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7}
	now := uint64(1_700_000_000_000_000_000)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "scheduler")}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Spans: []*tracepb.Span{{
					TraceId: traceId, SpanId: rootSpanId,
					Name: "bookings:send-reminders", Kind: tracepb.Span_SPAN_KIND_INTERNAL,
					StartTimeUnixNano: now, EndTimeUnixNano: now + 2_000_000,
					Attributes: []*commonpb.KeyValue{strKV("console.command", "bookings:send-reminders")},
				}},
			}},
		}},
	}

	_, tasks, _, _, _, _ := convertTraces(context.Background(), testProjectId, req)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if !tasks[0].IsRoot {
		t.Errorf("expected task.IsRoot == true, got false")
	}
	if tasks[0].Id != otelTraceIDToUUID(traceId) {
		t.Errorf("expected task.Id == otelTraceIDToUUID(trace_id), got %s", tasks[0].Id)
	}
}

func TestConvertTraces_InlineGenAi_BecomesAiTrace(t *testing.T) {
	traceId := []byte{0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e, 0x3f}
	rootSpanId := []byte{0xE0, 0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7}
	llmSpanId := []byte{0xE8, 0xE9, 0xEA, 0xEB, 0xEC, 0xED, 0xEE, 0xEF}
	now := uint64(1_700_000_000_000_000_000)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "api")}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Spans: []*tracepb.Span{
					{TraceId: traceId, SpanId: rootSpanId, Name: "POST /chat", Kind: tracepb.Span_SPAN_KIND_SERVER, StartTimeUnixNano: now, EndTimeUnixNano: now + 10_000_000,
						Attributes: []*commonpb.KeyValue{strKV("http.request.method", "POST"), strKV("http.route", "/chat")}},
					{TraceId: traceId, SpanId: llmSpanId, ParentSpanId: rootSpanId, Name: "chat openai", Kind: tracepb.Span_SPAN_KIND_INTERNAL, StartTimeUnixNano: now, EndTimeUnixNano: now + 5_000_000,
						Attributes: []*commonpb.KeyValue{strKV("gen_ai.system", "openai"), strKV("gen_ai.request.model", "gpt-4")}},
				},
			}},
		}},
	}

	endpoints, _, _, _, aiTraces, _ := convertTraces(context.Background(), testProjectId, req)
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	if !endpoints[0].IsRoot {
		t.Errorf("expected endpoint.IsRoot == true")
	}
	wantEndpointId := otelTraceIDToUUID(traceId)
	if endpoints[0].Id != wantEndpointId {
		t.Errorf("expected endpoint.Id %s, got %s", wantEndpointId, endpoints[0].Id)
	}

	if len(aiTraces) != 1 {
		t.Fatalf("expected 1 ai_trace, got %d", len(aiTraces))
	}
	if aiTraces[0].IsRoot {
		t.Errorf("expected aiTrace.IsRoot == false")
	}
	wantAiId := otelSpanIDToUUID(llmSpanId)
	if aiTraces[0].Id != wantAiId {
		t.Errorf("expected aiTrace.Id %s, got %s", wantAiId, aiTraces[0].Id)
	}
	if aiTraces[0].DistributedTraceId == nil || *aiTraces[0].DistributedTraceId != wantEndpointId {
		t.Errorf("expected aiTrace.DistributedTraceId %s, got %v", wantEndpointId, aiTraces[0].DistributedTraceId)
	}
}

func TestConvertTraces_ExceptionOnConsumer_TraceTypeIsTask(t *testing.T) {
	traceId := []byte{0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f}
	producerSpanId := []byte{0xF0, 0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7}
	consumerSpanId := []byte{0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF}
	now := uint64(1_700_000_000_000_000_000)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "worker")}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Spans: []*tracepb.Span{{
					TraceId: traceId, SpanId: consumerSpanId, ParentSpanId: producerSpanId,
					Name: "process job", Kind: tracepb.Span_SPAN_KIND_CONSUMER,
					StartTimeUnixNano: now, EndTimeUnixNano: now + 1_000_000,
					Events: []*tracepb.Span_Event{{
						Name:         "exception",
						TimeUnixNano: now + 500_000,
						Attributes: []*commonpb.KeyValue{
							strKV("exception.type", "RuntimeError"),
							strKV("exception.message", "boom"),
						},
					}},
				}},
			}},
		}},
	}

	_, tasks, _, exceptions, _, _ := convertTraces(context.Background(), testProjectId, req)
	if len(tasks) != 1 || len(exceptions) != 1 {
		t.Fatalf("expected 1 task + 1 exception, got %d / %d", len(tasks), len(exceptions))
	}
	if exceptions[0].TraceType != "task" {
		t.Errorf("expected exception.TraceType == 'task', got %q", exceptions[0].TraceType)
	}
	if exceptions[0].TraceId == nil || *exceptions[0].TraceId != tasks[0].Id {
		t.Errorf("expected exception.TraceId == task.Id %s, got %v", tasks[0].Id, exceptions[0].TraceId)
	}
}

func TestConvertTraces_OrphanSpan_FallsBackToTraceId(t *testing.T) {
	// A non-root span whose parent is not in this batch and the span itself
	// isn't classified as an entity. Should land in the spans table with
	// trace_id == otelTraceIDToUUID(trace_id) (orphan fallback path).
	traceId := []byte{0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x5b, 0x5c, 0x5d, 0x5e, 0x5f}
	orphanParentId := []byte{0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67}
	orphanSpanId := []byte{0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77}
	now := uint64(1_700_000_000_000_000_000)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "worker")}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Spans: []*tracepb.Span{{
					TraceId: traceId, SpanId: orphanSpanId, ParentSpanId: orphanParentId,
					Name: "redis GET", Kind: tracepb.Span_SPAN_KIND_INTERNAL,
					StartTimeUnixNano: now, EndTimeUnixNano: now + 100_000,
				}},
			}},
		}},
	}

	_, _, spans, _, _, _ := convertTraces(context.Background(), testProjectId, req)
	if len(spans) != 1 {
		t.Fatalf("expected 1 span row, got %d", len(spans))
	}
	want := otelTraceIDToUUID(traceId)
	if spans[0].TraceId != want {
		t.Errorf("expected orphan span.TraceId %s (fallback to OTel trace_id), got %s", want, spans[0].TraceId)
	}
}

// --- Helpers ---

func makeAttrs(key, val string) []*commonpb.KeyValue {
	return []*commonpb.KeyValue{strKV(key, val)}
}

func strKV(key, val string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
	}
}

func strArrayKV(key string, vals ...string) *commonpb.KeyValue {
	items := make([]*commonpb.AnyValue, len(vals))
	for i, v := range vals {
		items[i] = &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}
	}
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: items}}}}
}

func intArrayKV(key string, vals ...int64) *commonpb.KeyValue {
	items := make([]*commonpb.AnyValue, len(vals))
	for i, v := range vals {
		items[i] = &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}
	}
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: items}}}}
}

func TestBuildHoneycombStackTrace(t *testing.T) {
	attrs := []*commonpb.KeyValue{
		strArrayKV("exception.structured_stacktrace.urls", "https://x/app.js", "https://x/app.js"),
		strArrayKV("exception.structured_stacktrace.functions", "foo", ""),
		intArrayKV("exception.structured_stacktrace.lines", 10, 20),
		intArrayKV("exception.structured_stacktrace.columns", 5, 7),
	}
	got, ok := buildHoneycombStackTrace("Error", "boom", attrs)
	if !ok {
		t.Fatal("expected ok=true when structured stacktrace present")
	}
	want := "Error: boom\nfoo()\n    https://x/app.js:10:5\n    https://x/app.js:20:7"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}

	if _, ok := buildHoneycombStackTrace("Error", "boom", makeAttrs("exception.stacktrace", "x")); ok {
		t.Error("expected ok=false when the structured field is absent")
	}
}

func TestConvertTraces_HoneycombJsExceptionSymbolicates(t *testing.T) {
	traceId := []byte{0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf}
	spanId := []byte{0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7}
	now := uint64(1_700_000_000_000_000_000)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
				strKV("service.name", "web"),
				strKV("telemetry.sdk.language", "webjs"),
			}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Scope: &commonpb.InstrumentationScope{Name: "@opentelemetry/instrumentation-fetch"},
				Spans: []*tracepb.Span{{
					TraceId: traceId, SpanId: spanId,
					Name: "GET /", Kind: tracepb.Span_SPAN_KIND_SERVER,
					StartTimeUnixNano: now, EndTimeUnixNano: now + 1_000_000,
					Attributes: makeAttrs("http.route", "/"),
					Events: []*tracepb.Span_Event{{
						Name:         "exception",
						TimeUnixNano: now + 500_000,
						Attributes: []*commonpb.KeyValue{
							strKV("exception.type", "Error"),
							strKV("exception.message", "user has no name"),
							strArrayKV("exception.structured_stacktrace.urls", "app.min.js", "app.min.js"),
							strArrayKV("exception.structured_stacktrace.functions", "n", ""),
							intArrayKV("exception.structured_stacktrace.lines", 1, 1),
							intArrayKV("exception.structured_stacktrace.columns", 63, 146),
						},
					}},
				}},
			}},
		}},
	}

	setFakeStore(t, nil)
	_, _, _, exceptions, _, _ := convertTraces(context.Background(), testProjectId, req)
	if len(exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(exceptions))
	}
	exc := exceptions[0]
	wantParsed := "Error: user has no name\nn()\n    app.min.js:1:63\n    app.min.js:1:146"
	if exc.StackTrace != wantParsed {
		t.Errorf("honeycomb parse:\n got %q\nwant %q", exc.StackTrace, wantParsed)
	}
	if exc.Attributes["telemetry.sdk.language"] != "webjs" {
		t.Errorf("expected stamped telemetry.sdk.language=webjs, got %q", exc.Attributes["telemetry.sdk.language"])
	}
}

func TestConvertTraces_JsExceptionResolvesWithSourceMap(t *testing.T) {
	projectId := uuid.MustParse("00000000-0000-0000-0000-0000000000ac")
	setFakeStore(t, map[string][]byte{
		services.SourceMapStorageKey(projectId, "minified.js.map"): []byte(testSourceMap),
	})

	traceId := []byte{0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf}
	spanId := []byte{0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7}
	now := uint64(1_700_000_000_000_000_000)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
				strKV("service.name", "web"),
				strKV("telemetry.sdk.language", "webjs"),
			}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Scope: &commonpb.InstrumentationScope{Name: "@opentelemetry/instrumentation-fetch"},
				Spans: []*tracepb.Span{{
					TraceId: traceId, SpanId: spanId,
					Name: "GET /", Kind: tracepb.Span_SPAN_KIND_SERVER,
					StartTimeUnixNano: now, EndTimeUnixNano: now + 1_000_000,
					Attributes: makeAttrs("http.route", "/"),
					Events: []*tracepb.Span_Event{{
						Name:         "exception",
						TimeUnixNano: now + 500_000,
						Attributes: []*commonpb.KeyValue{
							strKV("exception.type", "Error"),
							strKV("exception.message", "boom"),
							strKV("exception.stacktrace", "Error: boom\n    at t (https://cdn.example.com/assets/minified.js:1:11)"),
						},
					}},
				}},
			}},
		}},
	}

	_, _, _, exceptions, _, _ := convertTraces(context.Background(), projectId, req)
	if len(exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(exceptions))
	}
	if !strings.Contains(exceptions[0].StackTrace, "original.js") {
		t.Errorf("expected stack trace resolved via source map, got %q", exceptions[0].StackTrace)
	}
}

func TestIsJsTelemetry(t *testing.T) {
	jsCases := [][2]string{
		{"webjs", ""},
		{"nodejs", ""},
		{"javascript", ""},
		{"TypeScript", ""},
		{"", "@opentelemetry/instrumentation-express"},
		{"", "@vercel/otel"},
		{"", "@prisma/instrumentation"},
		{"", "next.js"},
		{"nodejs", "io.opentelemetry.tomcat-7.0"},
	}
	for _, c := range jsCases {
		if !isJsTelemetry(c[0], c[1]) {
			t.Errorf("isJsTelemetry(%q, %q) = false, want true", c[0], c[1])
		}
	}

	nonJsCases := [][2]string{
		{"", ""},
		{"java", "io.opentelemetry.spring-webmvc-6.0"},
		{"python", "opentelemetry.instrumentation.flask"},
		{"go", "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"},
		{"dotnet", "OpenTelemetry.Instrumentation.AspNetCore"},
		{"ruby", "OpenTelemetry::Instrumentation::Rack"},
		{"", "@noslash"},
	}
	for _, c := range nonJsCases {
		if isJsTelemetry(c[0], c[1]) {
			t.Errorf("isJsTelemetry(%q, %q) = true, want false", c[0], c[1])
		}
	}
}
