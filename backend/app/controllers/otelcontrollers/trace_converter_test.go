package otelcontrollers

import (
	"encoding/json"
	"flag"
	"os"
	"sort"
	"testing"

	"github.com/google/uuid"
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

			endpoints, _, spans, exceptions, aiTraces, aiConversations := convertTraces(testProjectId, req)

			// Check if all child spans share a trace ID with an endpoint or AI trace.
			// (After the Id=spanId refactor, the trace ID is in TraceId, not Id.)
			endpointIds := map[uuid.UUID]bool{}
			for _, ep := range endpoints {
				endpointIds[ep.TraceId] = true
			}
			for _, at := range aiTraces {
				endpointIds[at.TraceId] = true
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

	endpoints, _, spans, _, _, _ := convertTraces(testProjectId, req)

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	rootTraceId := endpoints[0].TraceId
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

// TestConvertTraces_HttpRootWithGenAiChild verifies the dual-write behavior for
// a gen_ai child span of an HTTP server root: one endpoint row at the root,
// one ai_trace row at the child, one span row (the child only), and any
// exception on the child gets trace_type=ai_trace.
func TestConvertTraces_HttpRootWithGenAiChild(t *testing.T) {
	rootSpanId := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	childSpanId := []byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
	traceIdBytes := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}
	now := uint64(1700000000000000000)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{strKV("service.name", "agent")},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           traceIdBytes,
								SpanId:            rootSpanId,
								Name:              "POST /api/chat",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: now,
								EndTimeUnixNano:   now + 5_000_000,
								Attributes: []*commonpb.KeyValue{
									strKV("http.request.method", "POST"),
									strKV("http.route", "/api/chat"),
								},
							},
							{
								TraceId:           traceIdBytes,
								SpanId:            childSpanId,
								ParentSpanId:      rootSpanId,
								Name:              "chat.completion",
								Kind:              tracepb.Span_SPAN_KIND_CLIENT,
								StartTimeUnixNano: now + 1_000_000,
								EndTimeUnixNano:   now + 4_000_000,
								Attributes: []*commonpb.KeyValue{
									strKV("gen_ai.request.model", "gpt-4o"),
									strKV("gen_ai.system", "openai"),
								},
								Events: []*tracepb.Span_Event{
									{
										Name:         "exception",
										TimeUnixNano: now + 3_000_000,
										Attributes: []*commonpb.KeyValue{
											strKV("exception.type", "RateLimitError"),
											strKV("exception.message", "429"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	endpoints, tasks, spans, exceptions, aiTraces, _ := convertTraces(testProjectId, req)

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
	if len(aiTraces) != 1 {
		t.Fatalf("expected 1 ai_trace, got %d", len(aiTraces))
	}
	if len(spans) != 1 {
		t.Fatalf("expected 1 span (child only — root is single-write), got %d", len(spans))
	}
	if len(exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(exceptions))
	}

	ep := endpoints[0]
	ai := aiTraces[0]
	traceUUID := otelTraceIDToUUID(traceIdBytes)
	rootSpanUUID := otelSpanIDToUUID(rootSpanId)
	childSpanUUID := otelSpanIDToUUID(childSpanId)

	if ep.Id != rootSpanUUID {
		t.Errorf("endpoint.Id = %s, want root spanId %s", ep.Id, rootSpanUUID)
	}
	if ep.TraceId != traceUUID {
		t.Errorf("endpoint.TraceId = %s, want %s", ep.TraceId, traceUUID)
	}
	if ep.ParentSpanId != nil {
		t.Errorf("endpoint.ParentSpanId = %v, want nil for root", ep.ParentSpanId)
	}

	if ai.Id != childSpanUUID {
		t.Errorf("ai_trace.Id = %s, want child spanId %s", ai.Id, childSpanUUID)
	}
	if ai.TraceId != traceUUID {
		t.Errorf("ai_trace.TraceId = %s, want %s", ai.TraceId, traceUUID)
	}
	if ai.ParentSpanId == nil || *ai.ParentSpanId != rootSpanUUID {
		t.Errorf("ai_trace.ParentSpanId = %v, want %s", ai.ParentSpanId, rootSpanUUID)
	}

	if spans[0].Id != childSpanUUID {
		t.Errorf("span row should be the child, got Id=%s", spans[0].Id)
	}

	// Phase 2: the gen_ai child is itself an ai_trace entity, so its dual-written
	// span row should own itself (entity_id = childSpanUUID), NOT the endpoint
	// root. This is what keeps it out of the endpoint detail's waterfall.
	if spans[0].EntityId == nil || *spans[0].EntityId != childSpanUUID {
		t.Errorf("span.EntityId = %v, want %s (the gen_ai span itself)", spans[0].EntityId, childSpanUUID)
	}

	if exceptions[0].TraceType != "ai_trace" {
		t.Errorf("exception traceType = %q, want %q", exceptions[0].TraceType, "ai_trace")
	}
}

// TestConvertTraces_DispatchJob_EntityOwnership reproduces the Zentigo
// dispatch-job pattern: an HTTP endpoint root with a producer span child, plus
// a separate CONSUMER (task) sub-tree whose parent is the producer. The
// producer should belong to the endpoint entity, while the consumer and its
// children belong to the task entity. This is the test that proves the
// subtree-leak bug from Phase 1 is gone.
func TestConvertTraces_DispatchJob_EntityOwnership(t *testing.T) {
	rootSpanId := []byte{0xE0, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	producerSpanId := []byte{0xE0, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17}
	consumerSpanId := []byte{0xC0, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27}
	dbQuerySpanId := []byte{0xD0, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37}
	traceIdBytes := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}
	now := uint64(1700000000000000000)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "zentigo")}},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							// HTTP server span (endpoint root).
							{
								TraceId:           traceIdBytes,
								SpanId:            rootSpanId,
								Name:              "GET /test/traceway/dispatch-job",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: now,
								EndTimeUnixNano:   now + 10_000_000,
								Attributes: []*commonpb.KeyValue{
									strKV("http.request.method", "GET"),
									strKV("http.route", "/test/traceway/dispatch-job"),
								},
							},
							// Producer span inside the request (PRODUCER kind → generic).
							{
								TraceId:           traceIdBytes,
								SpanId:            producerSpanId,
								ParentSpanId:      rootSpanId,
								Name:              "queue.dispatch TestTracewayJob",
								Kind:              tracepb.Span_SPAN_KIND_PRODUCER,
								StartTimeUnixNano: now + 1_000_000,
								EndTimeUnixNano:   now + 2_000_000,
							},
							// CONSUMER span — separate sub-tree under the producer.
							{
								TraceId:           traceIdBytes,
								SpanId:            consumerSpanId,
								ParentSpanId:      producerSpanId,
								Name:              "TestTracewayJob",
								Kind:              tracepb.Span_SPAN_KIND_CONSUMER,
								StartTimeUnixNano: now + 5_000_000,
								EndTimeUnixNano:   now + 8_000_000,
							},
							// DB query inside handle().
							{
								TraceId:           traceIdBytes,
								SpanId:            dbQuerySpanId,
								ParentSpanId:      consumerSpanId,
								Name:              "select * from users",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: now + 6_000_000,
								EndTimeUnixNano:   now + 7_000_000,
							},
						},
					},
				},
			},
		},
	}

	endpoints, tasks, spans, _, _, _ := convertTraces(testProjectId, req)

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (the CONSUMER), got %d", len(tasks))
	}

	endpointSpanUUID := otelSpanIDToUUID(rootSpanId)
	producerUUID := otelSpanIDToUUID(producerSpanId)
	consumerUUID := otelSpanIDToUUID(consumerSpanId)
	dbQueryUUID := otelSpanIDToUUID(dbQuerySpanId)

	if endpoints[0].Id != endpointSpanUUID {
		t.Errorf("endpoint.Id = %s, want %s", endpoints[0].Id, endpointSpanUUID)
	}
	if tasks[0].Id != consumerUUID {
		t.Errorf("task.Id = %s, want consumer spanId %s", tasks[0].Id, consumerUUID)
	}

	// We expect 3 dual-written generic span rows: producer, consumer, db query.
	// (The root endpoint is single-write — no span row.)
	if len(spans) != 3 {
		t.Fatalf("expected 3 spans (producer, consumer, db query), got %d", len(spans))
	}

	gotEntity := map[uuid.UUID]uuid.UUID{} // spanId → entity_id
	for _, s := range spans {
		if s.EntityId == nil {
			t.Errorf("span %q has nil EntityId, expected resolution", s.Name)
			continue
		}
		gotEntity[s.Id] = *s.EntityId
	}

	if got := gotEntity[producerUUID]; got != endpointSpanUUID {
		t.Errorf("producer span EntityId = %s, want endpoint %s — producer should belong to the originating endpoint", got, endpointSpanUUID)
	}
	if got := gotEntity[consumerUUID]; got != consumerUUID {
		t.Errorf("consumer span EntityId = %s, want self %s — a CONSUMER span owns its own subtree", got, consumerUUID)
	}
	if got := gotEntity[dbQueryUUID]; got != consumerUUID {
		t.Errorf("db query EntityId = %s, want consumer %s — descendants of the task belong to the task entity", got, consumerUUID)
	}
}

// TestConvertTraces_MultiStepAgent verifies that an HTTP root with N gen_ai
// children produces N distinct ai_trace rows (each with its own spanId as Id).
func TestConvertTraces_MultiStepAgent(t *testing.T) {
	rootSpanId := []byte{0xA0, 0xA0, 0xA0, 0xA0, 0xA0, 0xA0, 0xA0, 0xA0}
	traceIdBytes := []byte{0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0, 0xC0}
	now := uint64(1700000000000000000)

	const N = 3
	scope := &tracepb.ScopeSpans{
		Spans: []*tracepb.Span{
			{
				TraceId:           traceIdBytes,
				SpanId:            rootSpanId,
				Name:              "POST /agent",
				Kind:              tracepb.Span_SPAN_KIND_SERVER,
				StartTimeUnixNano: now,
				EndTimeUnixNano:   now + 10_000_000,
				Attributes: []*commonpb.KeyValue{
					strKV("http.request.method", "POST"),
					strKV("http.route", "/agent"),
				},
			},
		},
	}
	for i := 0; i < N; i++ {
		childId := []byte{0xB0, 0xB0, 0xB0, 0xB0, 0xB0, 0xB0, 0xB0, byte(i + 1)}
		scope.Spans = append(scope.Spans, &tracepb.Span{
			TraceId:           traceIdBytes,
			SpanId:            childId,
			ParentSpanId:      rootSpanId,
			Name:              "step",
			Kind:              tracepb.Span_SPAN_KIND_CLIENT,
			StartTimeUnixNano: now + uint64(i)*1_000_000,
			EndTimeUnixNano:   now + uint64(i+1)*1_000_000,
			Attributes: []*commonpb.KeyValue{
				strKV("gen_ai.request.model", "gpt-4o"),
				strKV("gen_ai.system", "openai"),
			},
		})
	}

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource:   &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "agent")}},
				ScopeSpans: []*tracepb.ScopeSpans{scope},
			},
		},
	}

	endpoints, _, _, _, aiTraces, _ := convertTraces(testProjectId, req)

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	if len(aiTraces) != N {
		t.Fatalf("expected %d ai_trace rows, got %d", N, len(aiTraces))
	}

	seen := map[uuid.UUID]bool{}
	for _, ai := range aiTraces {
		if seen[ai.Id] {
			t.Errorf("duplicate ai_trace Id %s — each child should have its own row", ai.Id)
		}
		seen[ai.Id] = true
		if ai.ParentSpanId == nil {
			t.Errorf("ai_trace %s has nil ParentSpanId — should reference the root", ai.Id)
		}
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
