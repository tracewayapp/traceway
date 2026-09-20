//go:build !telemetry_ch && !telemetry_duckdb && !transactional_pg

package otelcontrollers

import (
	"bytes"
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestExportTracesPersistsLateParentGraphByDefault(t *testing.T) {
	dbtest.SetupSQLite(t)
	previousConfig := config.Config
	config.Config = &config.Cfg{}
	t.Cleanup(func() { config.Config = previousConfig })
	gin.SetMode(gin.TestMode)
	trace := uuid.New()
	now := uint64(time.Now().UnixNano())
	parent := &tracepb.Span{TraceId: trace[:], SpanId: []byte{1, 1, 1, 1, 1, 1, 1, 1}, Name: "worker", Kind: tracepb.Span_SPAN_KIND_CONSUMER, StartTimeUnixNano: now, EndTimeUnixNano: now + 1000}
	child := &tracepb.Span{TraceId: trace[:], SpanId: []byte{2, 2, 2, 2, 2, 2, 2, 2}, ParentSpanId: parent.SpanId, Name: "query", Kind: tracepb.Span_SPAN_KIND_INTERNAL, StartTimeUnixNano: now, EndTimeUnixNano: now + 500}
	invalid := &tracepb.Span{TraceId: []byte{1}, SpanId: child.SpanId}
	for i, request := range []*coltracepb.ExportTraceServiceRequest{spanRequest(child, invalid), spanRequest(parent)} {
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
		if recorder.Code != http.StatusOK || len(c.Errors) > 0 {
			t.Fatalf("export failed: %d %v", recorder.Code, c.Errors)
		}
		var response coltracepb.ExportTraceServiceResponse
		if err := proto.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if i == 0 && response.GetPartialSuccess().GetRejectedSpans() != 1 {
			t.Fatalf("missing invalid ID rejection: %v", &response)
		}
	}
	id := otelOccurrenceID(testProjectId, parent)
	task, err := telemetry.TaskRepository.FindById(context.Background(), testProjectId, id, nil)
	if err != nil || task == nil {
		t.Fatalf("missing task projection: %v", err)
	}
	if task.TraceId != hex.EncodeToString(trace[:]) || task.SpanId != "0101010101010101" || task.ParentSpanId != "" || !task.IsRoot {
		t.Fatalf("the task carries its span's ids: %+v", task)
	}
	graph, err := telemetry.SpanRepository.FindGraph(context.Background(), shared.NewSpanLookup(testProjectId, task.TraceId, task.SpanId, task.RecordedAt))
	if err != nil {
		t.Fatal(err)
	}
	if spans := graph.Spans; len(spans) != 1 || spans[0].Name != "query" {
		t.Fatalf("lost separately exported child: %+v", spans)
	}
	var stored int
	if err := db.TelemetryDB.QueryRow("SELECT COUNT(*) FROM spans_v2").Scan(&stored); err != nil || stored != 2 {
		t.Fatalf("default ingestion did not retain both source spans: %d %v", stored, err)
	}
}
