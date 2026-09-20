//go:build !telemetry_ch && !telemetry_duckdb && !transactional_pg

package otelcontrollers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

var traceTables = []string{"spans_v2", "endpoints_v2", "tasks_v2", "exceptions_v2", "ai_traces_v2"}

func traceTableCounts(t *testing.T) []int {
	t.Helper()
	counts := make([]int, len(traceTables))
	for i, table := range traceTables {
		if err := db.TelemetryDB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&counts[i]); err != nil {
			t.Fatal(err)
		}
	}
	return counts
}

func everyTableRequest() (*coltracepb.ExportTraceServiceRequest, *tracepb.Span, *tracepb.Span) {
	trace := uuid.New()
	now := uint64(time.Now().UnixNano())
	endpoint := &tracepb.Span{TraceId: trace[:], SpanId: []byte{1, 0, 0, 0, 0, 0, 0, 1}, Name: "GET /orders", Kind: tracepb.Span_SPAN_KIND_SERVER,
		StartTimeUnixNano: now, EndTimeUnixNano: now + 9000, Attributes: []*commonpb.KeyValue{strKV("http.request.method", "GET"), strKV("http.route", "/orders")}}
	child := &tracepb.Span{TraceId: trace[:], SpanId: []byte{2, 0, 0, 0, 0, 0, 0, 2}, ParentSpanId: endpoint.SpanId, Name: "query", Kind: tracepb.Span_SPAN_KIND_INTERNAL,
		StartTimeUnixNano: now + 1, EndTimeUnixNano: now + 500, Events: []*tracepb.Span_Event{{Name: "exception", TimeUnixNano: now + 2,
			Attributes: []*commonpb.KeyValue{strKV("exception.type", "PgError"), strKV("exception.message", "deadlock detected")}}}}
	task := &tracepb.Span{TraceId: trace[:], SpanId: []byte{3, 0, 0, 0, 0, 0, 0, 3}, ParentSpanId: endpoint.SpanId, Name: "process job", Kind: tracepb.Span_SPAN_KIND_CONSUMER,
		StartTimeUnixNano: now + 3, EndTimeUnixNano: now + 800}
	llm := &tracepb.Span{TraceId: trace[:], SpanId: []byte{4, 0, 0, 0, 0, 0, 0, 4}, ParentSpanId: endpoint.SpanId, Name: "chat model", Kind: tracepb.Span_SPAN_KIND_CLIENT,
		StartTimeUnixNano: now + 4, EndTimeUnixNano: now + 900, Attributes: []*commonpb.KeyValue{strKV("gen_ai.system", "anthropic"), strKV("gen_ai.request.model", "claude-sonnet-5")}}
	return spanRequest(endpoint, child, task, llm), endpoint, child
}

func exportTraces(t *testing.T, request *coltracepb.ExportTraceServiceRequest) (*httptest.ResponseRecorder, *coltracepb.ExportTraceServiceResponse) {
	t.Helper()
	payload, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/otel/v1/traces", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/x-protobuf")
	c.Set(middleware.ProjectIdContextKey, testProjectId)
	OtelController.ExportTraces(c)
	c.Writer.WriteHeaderNow()
	var response coltracepb.ExportTraceServiceResponse
	if recorder.Code == http.StatusOK {
		if err := proto.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
	}
	return recorder, &response
}

func setupTraceWrites(t *testing.T) {
	t.Helper()
	dbtest.SetupSQLite(t)
	previousConfig, previousStore := config.Config, traceStore
	config.Config = &config.Cfg{}
	setFakeStore(t, nil)
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { config.Config, traceStore = previousConfig, previousStore })
}

func TestExportTracesFailureAtEachWriteStage(t *testing.T) {
	stages := []struct {
		name      string
		inject    func(error)
		persisted []int
	}{
		{"spans", func(err error) {
			traceStore.spans = func(context.Context, []models.OtelSpan) ([]shared.SpanOwner, error) { return nil, err }
		}, []int{0, 0, 0, 0, 0}},
		{"endpoints", func(err error) { traceStore.endpoints = func(context.Context, []models.Endpoint) error { return err } }, []int{4, 0, 0, 0, 0}},
		{"tasks", func(err error) { traceStore.tasks = func(context.Context, []models.Task) error { return err } }, []int{4, 1, 0, 0, 0}},
		{"exceptions", func(err error) {
			traceStore.exceptions = func(context.Context, []models.ExceptionStackTrace) error { return err }
		}, []int{4, 1, 1, 0, 0}},
		{"ai traces", func(err error) { traceStore.aiTraces = func(context.Context, []models.AiTrace) error { return err } }, []int{4, 1, 1, 1, 0}},
	}
	failures := []struct {
		err        error
		status     int
		retryAfter string
	}{
		{errors.New("no such column: resource_pb"), http.StatusInternalServerError, ""},
		{fmt.Errorf("insert timed out: %w", context.DeadlineExceeded), http.StatusServiceUnavailable, "2"},
	}
	for _, stage := range stages {
		for _, failure := range failures {
			t.Run(fmt.Sprintf("%s/%d", stage.name, failure.status), func(t *testing.T) {
				setupTraceWrites(t)
				stage.inject(failure.err)
				request, _, _ := everyTableRequest()
				recorder, _ := exportTraces(t, request)
				if recorder.Code != failure.status || recorder.Header().Get("Retry-After") != failure.retryAfter {
					t.Fatalf("status %d Retry-After %q, want %d %q", recorder.Code, recorder.Header().Get("Retry-After"), failure.status, failure.retryAfter)
				}
				if got := traceTableCounts(t); fmt.Sprint(got) != fmt.Sprint(stage.persisted) {
					t.Fatalf("persisted rows %v, want %v in %v", got, stage.persisted, traceTables)
				}
			})
		}
	}
}

func TestExportTracesRetryDuplicatesAnalyticsButNotTheGraph(t *testing.T) {
	setupTraceWrites(t)
	request, endpoint, _ := everyTableRequest()
	for range 2 {
		if recorder, _ := exportTraces(t, request); recorder.Code != http.StatusOK {
			t.Fatalf("export failed: %d", recorder.Code)
		}
	}
	// Writes are separate and append-only, so a retried export is stored twice. Only graph reads deduplicate.
	if got := traceTableCounts(t); fmt.Sprint(got) != fmt.Sprint([]int{8, 2, 2, 2, 2}) {
		t.Fatalf("a retried export is at-least-once: got %v in %v", got, traceTables)
	}
	id := otelOccurrenceID(testProjectId, endpoint)
	stored, err := telemetry.EndpointRepository.FindById(context.Background(), testProjectId, id, nil)
	if err != nil || stored == nil {
		t.Fatalf("missing endpoint: %v", err)
	}
	graph, err := telemetry.SpanRepository.FindGraph(context.Background(), shared.NewSpanLookup(testProjectId, stored.TraceId, stored.SpanId, stored.RecordedAt))
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Spans) != 3 {
		t.Fatalf("graph reads must deduplicate the retry: %d spans", len(graph.Spans))
	}
}

func TestExportTracesReportsRowsStorageRejected(t *testing.T) {
	setupTraceWrites(t)
	store := traceStore.spans
	traceStore.spans = func(ctx context.Context, spans []models.OtelSpan) ([]shared.SpanOwner, error) {
		stored, err := store(ctx, spans[:len(spans)-2])
		for _, span := range spans[len(spans)-2:] {
			stored = append(stored, shared.SpanOwner{ProjectId: span.ProjectId, TraceId: span.TraceId, SpanId: span.SpanId})
		}
		return stored, err
	}
	request, _, _ := everyTableRequest()
	request.ResourceSpans[0].ScopeSpans[0].Spans = append(request.ResourceSpans[0].ScopeSpans[0].Spans, &tracepb.Span{TraceId: []byte{1}, SpanId: []byte{1}})
	recorder, response := exportTraces(t, request)
	partial := response.GetPartialSuccess()
	if recorder.Code != http.StatusOK || partial.GetRejectedSpans() != 3 {
		t.Fatalf("status %d, partial success %v", recorder.Code, partial)
	}
	if got := traceTableCounts(t); fmt.Sprint(got) != fmt.Sprint([]int{2, 1, 0, 1, 0}) {
		t.Fatalf("rejected task/AI spans must not produce projections: %v", got)
	}

	if !strings.Contains(partial.GetErrorMessage(), "1 spans need valid non-zero trace and span IDs") || !strings.Contains(partial.GetErrorMessage(), "2 spans could not be stored") {
		t.Fatalf("the exporter must learn why spans were rejected: %q", partial.GetErrorMessage())
	}
}
