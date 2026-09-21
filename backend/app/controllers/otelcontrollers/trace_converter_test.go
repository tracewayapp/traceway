package otelcontrollers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/controllers/clientcontrollers"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/services"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
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

func spanKey(traceId, spanId string) string { return traceId + ":" + spanId }

// nearestPromotedAncestor walks the stored parent edges the way a read does, and returns the span id of the first
// promoted span it meets.
func nearestPromotedAncestor(canonical []models.OtelSpan, span models.OtelSpan, promoted map[string]bool) (string, bool) {
	byKey := make(map[string]models.OtelSpan, len(canonical))
	for _, candidate := range canonical {
		byKey[spanKey(candidate.TraceId, candidate.SpanId)] = candidate
	}
	for range len(canonical) + 1 {
		if promoted[spanKey(span.TraceId, span.SpanId)] {
			return span.SpanId, true
		}
		if span.ParentSpanId == "" {
			return "", false
		}
		parent, ok := byKey[spanKey(span.TraceId, span.ParentSpanId)]
		if !ok {
			return "", false
		}
		span = parent
	}
	return "", false
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

	ConversationId string   `json:"conversationId"`
	ToolCallCount  int64    `json:"toolCallCount"`
	ToolNames      []string `json:"toolNames"`
	Flagged        bool     `json:"flagged"`
	FlaggedTerms   []string `json:"flaggedTerms"`
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
		{"openai_tool_calls_ai_trace", "testdata/openai_tool_calls_ai_trace.json"},
		{"anthropic_tool_use_ai_trace", "testdata/anthropic_tool_use_ai_trace.json"},
		{"node_better_auth", "testdata/node_better_auth.json"},
		{"node_sign_in", "testdata/node_sign_in.json"},
		{"spring_boot_exception", "testdata/spring_boot_exception.json"},
		{"honeycomb_global_error", "testdata/honeycomb_global_error.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := os.ReadFile(tt.fixture)
			if err != nil {
				t.Fatalf("failed to read fixture %s: %v", tt.fixture, err)
			}

			req := &coltracepb.ExportTraceServiceRequest{}
			normalized, err := normalizeOTLPJSON(raw, req.ProtoReflect().Descriptor())
			if err != nil {
				t.Fatal(err)
			}
			if err := protojson.Unmarshal(normalized, req); err != nil {
				t.Fatalf("failed to unmarshal fixture %s: %v", tt.fixture, err)
			}

			setFakeStore(t, nil)
			converted := convertTraces(context.Background(), nil, testProjectId, req)
			endpoints, exceptions, aiTraces, aiConversations := converted.Endpoints, converted.Exceptions, converted.AiTraces, converted.AiConversations
			spans := converted.Spans

			endpointIds := map[string]bool{}
			for _, ep := range endpoints {
				endpointIds[spanKey(ep.TraceId, ep.SpanId)] = true
			}
			for _, at := range aiTraces {
				endpointIds[spanKey(at.TraceId, at.SpanId)] = true
			}
			allLinked := true
			snapSpans := make([]snapshotSpan, len(spans))
			for i, s := range spans {
				_, linked := nearestPromotedAncestor(spans, s, endpointIds)
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
					ConversationId:  at.ConversationId,
					ToolCallCount:   at.ToolCallCount,
					ToolNames:       at.ToolNames,
					Flagged:         at.Flagged,
					FlaggedTerms:    at.FlaggedTerms,
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

func TestBuildEndpoint404Unmatched(t *testing.T) {
	tests := []struct {
		name  string
		attrs []*commonpb.KeyValue
		want  string
	}{
		{"matched route keeps identity on 404", []*commonpb.KeyValue{
			strKV("http.request.method", "GET"),
			strKV("http.route", "/users/:id"),
			intKV("http.response.status_code", 404),
		}, "GET /users/:id"},
		{"no route collapses on 404", []*commonpb.KeyValue{
			strKV("http.request.method", "GET"),
			strKV("url.path", "/wp-admin.php"),
			intKV("http.response.status_code", 404),
		}, "UNMATCHED"},
		{"invalid route collapses on 404", []*commonpb.KeyValue{
			strKV("http.request.method", "GET"),
			strKV("http.route", "no-slash"),
			intKV("http.response.status_code", 404),
		}, "UNMATCHED"},
		{"express middleware root route collapses on 404", []*commonpb.KeyValue{
			strKV("http.request.method", "GET"),
			strKV("http.route", "/"),
			intKV("http.response.status_code", 404),
		}, "UNMATCHED"},
		{"wildcard route collapses on 404", []*commonpb.KeyValue{
			strKV("http.request.method", "GET"),
			strKV("http.route", "/*"),
			intKV("http.response.status_code", 404),
		}, "UNMATCHED"},
		{"spring resource handler route collapses on 404", []*commonpb.KeyValue{
			strKV("http.request.method", "GET"),
			strKV("http.route", "/**"),
			intKV("http.response.status_code", 404),
		}, "UNMATCHED"},
		{"scoped wildcard route kept on 404", []*commonpb.KeyValue{
			strKV("http.request.method", "GET"),
			strKV("http.route", "/api/*"),
			intKV("http.response.status_code", 404),
		}, "GET /api/*"},
		{"matched route on 200 unaffected", []*commonpb.KeyValue{
			strKV("http.request.method", "GET"),
			strKV("http.route", "/users/:id"),
			intKV("http.response.status_code", 200),
		}, "GET /users/:id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := &tracepb.Span{Name: "fallback"}
			ep := buildEndpoint(uuid.New(), testProjectId, span, tt.attrs, nil, time.Time{}, 0, "", "")
			if ep.Endpoint != tt.want {
				t.Errorf("buildEndpoint().Endpoint = %q, want %q", ep.Endpoint, tt.want)
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
								TraceId:           traceIdBytes,
								SpanId:            rootSpanId,
								Name:              "GET /api/test",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
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

	converted := convertTraces(context.Background(), nil, testProjectId, req)
	endpoints := converted.Endpoints

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	spans := converted.Spans
	if len(spans) < 2 {
		t.Fatalf("expected the root and its descendants to be stored, got %d spans", len(spans))
	}
	promoted := map[string]bool{spanKey(endpoints[0].TraceId, endpoints[0].SpanId): true}
	for _, s := range spans {
		if owner, ok := nearestPromotedAncestor(spans, s, promoted); !ok || owner != endpoints[0].SpanId {
			t.Errorf("span %q does not reach the root endpoint through its stored parent edges", s.Name)
		}
	}
}

func TestFormatExceptionStackTrace(t *testing.T) {
	tests := []struct {
		name          string
		excType       string
		excMessage    string
		excStacktrace string
		want          string
	}{
		{
			name:          "go style - no stacktrace",
			excType:       "RuntimeError",
			excMessage:    "something failed",
			excStacktrace: "",
			want:          "RuntimeError: something failed",
		},
		{
			name:          "go style - with stacktrace that doesn't start with type",
			excType:       "RuntimeError",
			excMessage:    "something failed",
			excStacktrace: "goroutine 1 [running]:\nmain.foo()\n\t/app/main.go:10",
			want:          "RuntimeError: something failed\ngoroutine 1 [running]:\nmain.foo()\n\t/app/main.go:10",
		},
		{
			// Java/JVM OTel agents include the full "Type: message\n\tat ..." in
			// exception.stacktrace, so we must not prepend a duplicate header.
			name:          "java style - stacktrace already starts with exception type",
			excType:       "org.springframework.dao.EmptyResultDataAccessException",
			excMessage:    "Incorrect result size: expected 1, actual 0",
			excStacktrace: "org.springframework.dao.EmptyResultDataAccessException: Incorrect result size: expected 1, actual 0\n\tat org.springframework.dao.support.DataAccessUtils.requiredSingleResult(DataAccessUtils.java:90)\n\tat com.example.UserService.getUser(UserService.java:38)",
			want:          "org.springframework.dao.EmptyResultDataAccessException: Incorrect result size: expected 1, actual 0\n\tat org.springframework.dao.support.DataAccessUtils.requiredSingleResult(DataAccessUtils.java:90)\n\tat com.example.UserService.getUser(UserService.java:38)",
		},
		{
			name:          "java style - type only, no message",
			excType:       "java.lang.NullPointerException",
			excMessage:    "",
			excStacktrace: "java.lang.NullPointerException\n\tat com.example.Service.run(Service.java:10)",
			want:          "java.lang.NullPointerException\n\tat com.example.Service.run(Service.java:10)",
		},
		{
			name:          "empty everything",
			excType:       "",
			excMessage:    "",
			excStacktrace: "",
			want:          "unknown exception",
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

	converted := convertTraces(context.Background(), nil, testProjectId, req)
	endpoints, tasks := converted.Endpoints, converted.Tasks

	if len(endpoints) != 0 {
		t.Fatalf("expected 0 endpoints, got %d", len(endpoints))
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].IsRoot {
		t.Errorf("expected task.IsRoot == false, got true")
	}
	wantTaskId := otelOccurrenceID(testProjectId, &tracepb.Span{TraceId: traceId, SpanId: consumerSpanId})
	if tasks[0].Id != wantTaskId {
		t.Errorf("expected task.Id %s (from span_id), got %s", wantTaskId, tasks[0].Id)
	}
	if tasks[0].TraceId != hex.EncodeToString(traceId) || tasks[0].SpanId != hex.EncodeToString(consumerSpanId) || tasks[0].ParentSpanId == "" {
		t.Errorf("expected the task to carry its span's own ids, got %+v", tasks[0])
	}

	spans := converted.Spans
	if len(spans) != 2 {
		t.Fatalf("expected the consumer and its child to be stored, got %d", len(spans))
	}
	child := spans[1]
	if owner, ok := nearestPromotedAncestor(spans, child, map[string]bool{spanKey(tasks[0].TraceId, tasks[0].SpanId): true}); !ok || owner != tasks[0].SpanId {
		t.Errorf("expected the child to reach task %s through its stored parent edge", wantTaskId)
	}
	if getStringAttribute(child.OTLP.Attributes, "db.system") != "postgresql" {
		t.Errorf("expected child span attribute db.system 'postgresql', got %q", getStringAttribute(child.OTLP.Attributes, "db.system"))
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

	tasks := convertTraces(context.Background(), nil, testProjectId, req).Tasks
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if !tasks[0].IsRoot {
		t.Errorf("expected task.IsRoot == true, got false")
	}
	if tasks[0].Id != otelOccurrenceID(testProjectId, &tracepb.Span{TraceId: traceId, SpanId: rootSpanId}) {
		t.Errorf("expected the stable task occurrence ID, got %s", tasks[0].Id)
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

	converted := convertTraces(context.Background(), nil, testProjectId, req)
	endpoints, aiTraces := converted.Endpoints, converted.AiTraces
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	if !endpoints[0].IsRoot {
		t.Errorf("expected endpoint.IsRoot == true")
	}
	wantEndpointId := otelOccurrenceID(testProjectId, &tracepb.Span{TraceId: traceId, SpanId: rootSpanId})
	if endpoints[0].Id != wantEndpointId {
		t.Errorf("expected endpoint.Id %s, got %s", wantEndpointId, endpoints[0].Id)
	}

	if len(aiTraces) != 1 {
		t.Fatalf("expected 1 ai_trace, got %d", len(aiTraces))
	}
	if aiTraces[0].IsRoot {
		t.Errorf("expected aiTrace.IsRoot == false")
	}
	wantAiId := otelOccurrenceID(testProjectId, &tracepb.Span{TraceId: traceId, SpanId: llmSpanId})
	if aiTraces[0].Id != wantAiId {
		t.Errorf("expected aiTrace.Id %s, got %s", wantAiId, aiTraces[0].Id)
	}
	if aiTraces[0].TraceId != hex.EncodeToString(traceId) || aiTraces[0].SpanId != hex.EncodeToString(llmSpanId) || aiTraces[0].ParentSpanId == "" {
		t.Errorf("expected the AI trace to carry its span's own ids, got %+v", aiTraces[0])
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

	converted := convertTraces(context.Background(), nil, testProjectId, req)
	tasks, exceptions := converted.Tasks, converted.Exceptions
	if len(tasks) != 1 || len(exceptions) != 1 {
		t.Fatalf("expected 1 task + 1 exception, got %d / %d", len(tasks), len(exceptions))
	}
	if exceptions[0].TraceType != "task" {
		t.Errorf("expected exception.TraceType == 'task', got %q", exceptions[0].TraceType)
	}
	if exceptions[0].TraceId != tasks[0].TraceId || exceptions[0].SpanId != tasks[0].SpanId {
		t.Errorf("expected the exception on the task's own span, got trace %s span %s", exceptions[0].TraceId, exceptions[0].SpanId)
	}
}

func TestConvertTraces_OrphanSpan_KeepsSourceIdentity(t *testing.T) {
	// A non-root span whose parent is not in this batch and the span itself
	// isn't classified as an entity. It is stored with its source trace and
	// parent IDs untouched, so the edge connects when the parent arrives.
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

	converted := convertTraces(context.Background(), nil, testProjectId, req)
	endpoints, tasks, aiTraces := converted.Endpoints, converted.Tasks, converted.AiTraces
	if len(endpoints) != 0 || len(tasks) != 0 || len(aiTraces) != 0 {
		t.Fatalf("an unclassified orphan must not be promoted: %d endpoints / %d tasks / %d aiTraces", len(endpoints), len(tasks), len(aiTraces))
	}
	spans := converted.Spans
	if len(spans) != 1 {
		t.Fatalf("expected 1 stored span, got %d", len(spans))
	}
	if spans[0].TraceId != hex.EncodeToString(traceId) || spans[0].SpanId != hex.EncodeToString(orphanSpanId) {
		t.Errorf("source identity changed: trace %s span %s", spans[0].TraceId, spans[0].SpanId)
	}
	if spans[0].ParentSpanId != hex.EncodeToString(orphanParentId) {
		t.Errorf("expected the source parent ID to be kept, got %v", spans[0].ParentSpanId)
	}
}

func TestConvertTraces_ExceptionWithoutItsEndpointKeepsItsSpan(t *testing.T) {
	traceId := []byte{0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e, 0x3f, 0x40}
	rootId := []byte{1, 1, 1, 1, 1, 1, 1, 1}
	childId := []byte{2, 2, 2, 2, 2, 2, 2, 2}
	now := uint64(1_700_000_000_000_000_000)
	child := &tracepb.Span{TraceId: traceId, SpanId: childId, ParentSpanId: rootId, Name: "work", Kind: tracepb.Span_SPAN_KIND_INTERNAL,
		StartTimeUnixNano: now, EndTimeUnixNano: now + 1_000_000,
		Events: []*tracepb.Span_Event{{Name: "exception", TimeUnixNano: now + 500_000, Attributes: []*commonpb.KeyValue{
			strKV("exception.type", "RuntimeError"), strKV("exception.message", "boom"), strKV("exception.stacktrace", "RuntimeError: boom\n  at work (app.js:10:5)"),
		}}}}

	// The child ends first, so an exporter flush between the two leaves it without its endpoint.
	exceptions := convertTraces(context.Background(), nil, testProjectId, spanRequest(child)).Exceptions
	if len(exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(exceptions))
	}
	// Its endpoint is not in the payload and ingest does not go looking. The trace and the span are all a read needs.
	if exceptions[0].TraceId != hex.EncodeToString(traceId) || exceptions[0].SpanId != hex.EncodeToString(childId) || exceptions[0].TraceType != "" {
		t.Fatalf("expected the exception's own trace and span and no kind, got %+v", exceptions[0])
	}
	for key := range exceptions[0].Attributes {
		if strings.HasPrefix(key, "traceway.") {
			t.Fatalf("ingest must not add attributes of its own: %v", exceptions[0].Attributes)
		}
	}
}

func TestConvertTraces_NestedServerSpanIsOneRequest(t *testing.T) {
	traceId := []byte{0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30}
	outerId := []byte{1, 1, 1, 1, 1, 1, 1, 1}
	innerId := []byte{2, 2, 2, 2, 2, 2, 2, 2}
	const hasIsRemote = uint32(tracepb.SpanFlags_SPAN_FLAGS_CONTEXT_HAS_IS_REMOTE_MASK)
	const isRemote = uint32(tracepb.SpanFlags_SPAN_FLAGS_CONTEXT_IS_REMOTE_MASK)
	http := []*commonpb.KeyValue{strKV("http.request.method", "GET"), strKV("http.route", "/users")}
	now := uint64(1_700_000_000_000_000_000)
	server := func(id, parent []byte, flags uint32) *tracepb.Span {
		return &tracepb.Span{TraceId: traceId, SpanId: id, ParentSpanId: parent, Name: "GET /users", Kind: tracepb.Span_SPAN_KIND_SERVER,
			Attributes: http, Flags: flags, StartTimeUnixNano: now, EndTimeUnixNano: now + 1_000_000}
	}
	middleId := []byte{3, 3, 3, 3, 3, 3, 3, 3}
	wrapper := func(id, parent []byte) *tracepb.Span {
		return &tracepb.Span{TraceId: traceId, SpanId: id, ParentSpanId: parent, Name: "handle", Kind: tracepb.Span_SPAN_KIND_INTERNAL, StartTimeUnixNano: now, EndTimeUnixNano: now + 1_000_000}
	}
	client := &tracepb.Span{TraceId: traceId, SpanId: outerId, Name: "GET", Kind: tracepb.Span_SPAN_KIND_CLIENT, StartTimeUnixNano: now, EndTimeUnixNano: now + 1_000_000}

	tests := []struct {
		name  string
		spans []*tracepb.Span
		want  int
	}{
		{"local parent by flags, same batch", []*tracepb.Span{server(outerId, nil, 0), server(innerId, outerId, hasIsRemote)}, 1},
		{"local parent by flags, unknown kind in another batch", []*tracepb.Span{server(innerId, outerId, hasIsRemote)}, 1},
		{"remote parent by flags, same batch", []*tracepb.Span{server(outerId, nil, 0), server(innerId, outerId, hasIsRemote|isRemote)}, 2},
		{"no flags, parent in the same batch", []*tracepb.Span{server(outerId, nil, 0), server(innerId, outerId, 0)}, 1},
		{"no flags, parent in another batch", []*tracepb.Span{server(innerId, outerId, 0)}, 1},
		{"no flags, calling CLIENT span in the same batch", []*tracepb.Span{client, server(innerId, outerId, 0)}, 1},
		{"no flags, plain wrapper span above the request", []*tracepb.Span{wrapper(outerId, nil), server(innerId, outerId, 0)}, 1},
		{"local by flags, plain wrapper span above the request", []*tracepb.Span{wrapper(outerId, nil), server(innerId, outerId, hasIsRemote)}, 1},
		{"nested through a plain span in between", []*tracepb.Span{server(outerId, nil, 0), wrapper(middleId, outerId), server(innerId, middleId, hasIsRemote)}, 1},
		{"parent cycle does not hang", []*tracepb.Span{wrapper(outerId, middleId), wrapper(middleId, outerId), server(innerId, middleId, 0)}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := convertTraces(context.Background(), nil, testProjectId, spanRequest(tt.spans...)).Endpoints
			if len(endpoints) != tt.want {
				t.Fatalf("expected %d endpoints, got %d", tt.want, len(endpoints))
			}
			if stored := convertTraces(context.Background(), nil, testProjectId, spanRequest(tt.spans...)).Spans; len(stored) != len(tt.spans) {
				t.Fatalf("every span must still be stored, got %d of %d", len(stored), len(tt.spans))
			}
		})
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

func intKV(key string, val int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: val}},
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
	exceptions := convertTraces(context.Background(), nil, testProjectId, req).Exceptions
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

	exceptions := convertTraces(context.Background(), tokenProject(projectId), projectId, req).Exceptions
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

func honeycombExceptionSpanAttrs() []*commonpb.KeyValue {
	return []*commonpb.KeyValue{
		strKV("exception.type", "TypeError"),
		strKV("exception.message", "discount rate must be finite"),
		strKV("exception.stacktrace", "TypeError: discount rate must be finite\n    at assertValid (http://localhost:4173/assets/app.js:1:730)\n    at handleCheckout (http://localhost:4173/assets/app.js:1:1366)"),
		strArrayKV("exception.structured_stacktrace.urls", "http://localhost:4173/assets/app.js", "http://localhost:4173/assets/app.js"),
		strArrayKV("exception.structured_stacktrace.functions", "assertValid", "handleCheckout"),
		intArrayKV("exception.structured_stacktrace.lines", 1, 1),
		intArrayKV("exception.structured_stacktrace.columns", 730, 1366),
	}
}

func honeycombExceptionSpanRequest(spanAttrs []*commonpb.KeyValue, events []*tracepb.Span_Event, resourceAttrs ...*commonpb.KeyValue) *coltracepb.ExportTraceServiceRequest {
	traceId := []byte{0xc0, 0xc1, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf}
	spanId := []byte{0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7}
	now := uint64(1_700_000_000_000_000_000)

	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: append([]*commonpb.KeyValue{strKV("service.name", "web")}, resourceAttrs...)},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Scope: &commonpb.InstrumentationScope{Name: "@honeycombio/instrumentation-global-errors"},
				Spans: []*tracepb.Span{{
					TraceId: traceId, SpanId: spanId,
					Name: "exception", Kind: tracepb.Span_SPAN_KIND_INTERNAL,
					StartTimeUnixNano: now, EndTimeUnixNano: now,
					Attributes: spanAttrs,
					Events:     events,
				}},
			}},
		}},
	}
}

func TestConvertTraces_ExceptionSpanAttrs_CapturedWithoutAKind(t *testing.T) {
	req := honeycombExceptionSpanRequest(honeycombExceptionSpanAttrs(), nil, strKV("telemetry.sdk.language", "webjs"))

	setFakeStore(t, nil)
	converted := convertTraces(context.Background(), nil, testProjectId, req)
	endpoints, tasks, exceptions := converted.Endpoints, converted.Tasks, converted.Exceptions
	if len(endpoints) != 0 || len(tasks) != 0 {
		t.Fatalf("expected no entity rows, got %d endpoints / %d tasks", len(endpoints), len(tasks))
	}
	if len(exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(exceptions))
	}
	exc := exceptions[0]
	if exc.TraceType != "" || exc.TraceId == "" || exc.SpanId == "" {
		t.Errorf("an exception on a span that is no endpoint or task keeps its trace and span and names no kind, got %+v", exc)
	}
	want := "TypeError: discount rate must be finite\nassertValid()\n    http://localhost:4173/assets/app.js:1:730\nhandleCheckout()\n    http://localhost:4173/assets/app.js:1:1366"
	if exc.StackTrace != want {
		t.Errorf("expected stack built from structured arrays:\n got %q\nwant %q", exc.StackTrace, want)
	}
	if _, ok := exc.Attributes["exception.stacktrace"]; ok {
		t.Error("expected exception.stacktrace stripped from exception attributes")
	}
}

func TestConvertTraces_ExceptionSpanAttrs_StrippedFromEndpointRow(t *testing.T) {
	traceId := []byte{0xe0, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef}
	spanId := []byte{0xe8, 0xe9, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef}
	now := uint64(1_700_000_000_000_000_000)

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
				strKV("service.name", "web"),
				strKV("telemetry.sdk.language", "webjs"),
			}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Scope: &commonpb.InstrumentationScope{Name: "@scope/server"},
				Spans: []*tracepb.Span{{
					TraceId: traceId, SpanId: spanId,
					Name: "POST /checkout", Kind: tracepb.Span_SPAN_KIND_SERVER,
					StartTimeUnixNano: now, EndTimeUnixNano: now + 1_000_000,
					Attributes: []*commonpb.KeyValue{
						strKV("http.route", "/checkout"),
						strKV("exception.type", "Error"),
						strKV("exception.message", "boom"),
						strKV("exception.stacktrace", "Error: boom\n    at handler (http://x/app.js:1:10)"),
					},
				}},
			}},
		}},
	}

	setFakeStore(t, nil)
	converted := convertTraces(context.Background(), nil, testProjectId, req)
	endpoints, exceptions := converted.Endpoints, converted.Exceptions
	if len(endpoints) != 1 || len(exceptions) != 1 {
		t.Fatalf("expected 1 endpoint + 1 exception, got %d / %d", len(endpoints), len(exceptions))
	}
	if exceptions[0].TraceType != "endpoint" {
		t.Errorf("expected TraceType 'endpoint', got %q", exceptions[0].TraceType)
	}
	if _, ok := endpoints[0].Attributes["exception.stacktrace"]; ok {
		t.Error("expected exception.stacktrace stripped from endpoint attributes")
	}
	if endpoints[0].Attributes["http.route"] != "/checkout" {
		t.Errorf("expected other endpoint attributes intact, got %v", endpoints[0].Attributes)
	}
}

func TestConvertTraces_ExceptionEventAndSpanAttrs_EventWins(t *testing.T) {
	now := uint64(1_700_000_000_000_000_000)
	events := []*tracepb.Span_Event{{
		Name:         "exception",
		TimeUnixNano: now,
		Attributes: []*commonpb.KeyValue{
			strKV("exception.type", "EventError"),
			strKV("exception.message", "from the event"),
		},
	}}
	req := honeycombExceptionSpanRequest(honeycombExceptionSpanAttrs(), events, strKV("telemetry.sdk.language", "webjs"))

	setFakeStore(t, nil)
	exceptions := convertTraces(context.Background(), nil, testProjectId, req).Exceptions
	if len(exceptions) != 1 {
		t.Fatalf("expected exactly 1 exception, got %d", len(exceptions))
	}
	if !strings.HasPrefix(exceptions[0].StackTrace, "EventError: from the event") {
		t.Errorf("expected the event to win, got %q", exceptions[0].StackTrace)
	}
}

func TestConvertTraces_ExceptionSpanAttrs_HeaderOnly(t *testing.T) {
	attrs := []*commonpb.KeyValue{
		strKV("exception.type", "Error"),
		strKV("exception.message", "Script error."),
	}
	req := honeycombExceptionSpanRequest(attrs, nil, strKV("telemetry.sdk.language", "webjs"))

	setFakeStore(t, nil)
	exceptions := convertTraces(context.Background(), nil, testProjectId, req).Exceptions
	if len(exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(exceptions))
	}
	if exceptions[0].StackTrace != "Error: Script error." {
		t.Errorf("expected header-only stack trace, got %q", exceptions[0].StackTrace)
	}
}

func TestConvertTraces_ExceptionSpanAttrs_NoLanguageAttr(t *testing.T) {
	req := honeycombExceptionSpanRequest(honeycombExceptionSpanAttrs(), nil)

	setFakeStore(t, nil)
	exceptions := convertTraces(context.Background(), nil, testProjectId, req).Exceptions
	if len(exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(exceptions))
	}
	want := "TypeError: discount rate must be finite\nassertValid()\n    http://localhost:4173/assets/app.js:1:730\nhandleCheckout()\n    http://localhost:4173/assets/app.js:1:1366"
	if exceptions[0].StackTrace != want {
		t.Errorf("expected JS canonical output via scope-name fallback:\n got %q\nwant %q", exceptions[0].StackTrace, want)
	}
}

func fourWayRequest(spanAttrs []*commonpb.KeyValue, eventAttrs []*commonpb.KeyValue) *coltracepb.ExportTraceServiceRequest {
	traceId := []byte{0xf0, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0xfb, 0xfc, 0xfd, 0xfe, 0x01}
	spanId := []byte{0xf0, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0x01}
	now := uint64(1_700_000_000_000_000_000)

	span := &tracepb.Span{
		TraceId: traceId, SpanId: spanId,
		Name: "exception", Kind: tracepb.Span_SPAN_KIND_INTERNAL,
		StartTimeUnixNano: now, EndTimeUnixNano: now,
		Attributes: spanAttrs,
	}
	if eventAttrs != nil {
		span.Events = []*tracepb.Span_Event{{Name: "exception", TimeUnixNano: now, Attributes: eventAttrs}}
	}
	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
				strKV("service.name", "web"),
				strKV("telemetry.sdk.language", "webjs"),
			}},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Scope: &commonpb.InstrumentationScope{Name: "@honeycombio/instrumentation-global-errors"},
				Spans: []*tracepb.Span{span},
			}},
		}},
	}
}

func TestExceptionHash_FourWayStability(t *testing.T) {
	setFakeStore(t, nil)

	sdkCanonical := "TypeError: discount rate is corrupted\nassertValidDiscountRate()\n    app.min.js:1:730\napplyDiscount()\n    app.min.js:1:199"
	hashSdk := clientcontrollers.ComputeExceptionHash(sdkCanonical, false)

	chromeRaw := []*commonpb.KeyValue{
		strKV("exception.type", "TypeError"),
		strKV("exception.message", "discount rate is corrupted"),
		strKV("exception.stacktrace", "TypeError: discount rate is corrupted\n    at assertValidDiscountRate (http://localhost:4173/assets/app.min.js:1:730)\n    at applyDiscount (http://localhost:4173/assets/app.min.js:1:199)"),
	}
	firefoxRaw := []*commonpb.KeyValue{
		strKV("exception.type", "TypeError"),
		strKV("exception.message", "discount rate is corrupted"),
		strKV("exception.stacktrace", "assertValidDiscountRate@http://localhost:4173/assets/app.min.js:1:730\napplyDiscount@http://localhost:4173/assets/app.min.js:1:199"),
	}
	structured := []*commonpb.KeyValue{
		strKV("exception.type", "TypeError"),
		strKV("exception.message", "discount rate is corrupted"),
		strArrayKV("exception.structured_stacktrace.urls", "http://localhost:4173/assets/app.min.js", "http://localhost:4173/assets/app.min.js"),
		strArrayKV("exception.structured_stacktrace.functions", "assertValidDiscountRate", "applyDiscount"),
		intArrayKV("exception.structured_stacktrace.lines", 1, 1),
		intArrayKV("exception.structured_stacktrace.columns", 730, 199),
	}

	hashes := map[string]string{"sdk-canonical": hashSdk}
	for name, attrs := range map[string][]*commonpb.KeyValue{
		"chrome-raw":  chromeRaw,
		"firefox-raw": firefoxRaw,
		"structured":  structured,
	} {
		exceptions := convertTraces(context.Background(), nil, testProjectId, fourWayRequest(attrs, nil)).Exceptions
		if len(exceptions) != 1 {
			t.Fatalf("%s: expected 1 exception, got %d", name, len(exceptions))
		}
		hashes[name] = exceptions[0].ExceptionHash
	}

	for name, h := range hashes {
		if h != hashSdk {
			t.Errorf("hash mismatch: %s = %s, sdk-canonical = %s (all four ingest forms of the same error must group into one issue)", name, h, hashSdk)
		}
	}
	if hashSdk != "ae66d509d719ab7a" {
		t.Errorf("documented four-way hash changed: got %s, want ae66d509d719ab7a (update this constant only if the grouping algorithm intentionally changed)", hashSdk)
	}
}

func TestConvertTraces_FrontendFrameworkSuppressesEntityRows(t *testing.T) {
	raw, err := os.ReadFile("testdata/honeycomb_global_error.json")
	if err != nil {
		t.Fatal(err)
	}
	req := &coltracepb.ExportTraceServiceRequest{}
	normalized, err := normalizeOTLPJSON(raw, req.ProtoReflect().Descriptor())
	if err != nil {
		t.Fatal(err)
	}
	if err := protojson.Unmarshal(normalized, req); err != nil {
		t.Fatal(err)
	}

	setFakeStore(t, nil)
	reactProject := &models.Project{Id: testProjectId, Framework: "react"}
	converted := convertTraces(context.Background(), reactProject, testProjectId, req)
	endpoints, tasks, exceptions, aiTraces := converted.Endpoints, converted.Tasks, converted.Exceptions, converted.AiTraces
	if len(endpoints) != 0 || len(tasks) != 0 || len(aiTraces) != 0 {
		t.Fatalf("expected no entity rows for a frontend-framework project, got %d endpoints / %d tasks / %d aiTraces",
			len(endpoints), len(tasks), len(aiTraces))
	}
	if len(exceptions) != 1 {
		t.Fatalf("expected the exception to still be extracted, got %d", len(exceptions))
	}
	if exceptions[0].TraceType != "" {
		t.Errorf("expected no kind when nothing was promoted, got %q", exceptions[0].TraceType)
	}

	backendProject := &models.Project{Id: testProjectId, Framework: "gin"}
	endpoints = convertTraces(context.Background(), backendProject, testProjectId, req).Endpoints
	if len(endpoints) == 0 {
		t.Error("expected non-frontend frameworks to keep promoting endpoint rows")
	}
}
