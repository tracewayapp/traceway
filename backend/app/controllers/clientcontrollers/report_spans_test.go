//go:build !telemetry_ch && !telemetry_duckdb && !transactional_pg

package clientcontrollers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models/clientmodels"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestReportKeepsNativeSpanEdgesAndHistoricalTimes(t *testing.T) {
	dbtest.SetupSQLite(t)
	project, owner, parent, child := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	recorded := time.Now().UTC().Add(-72 * time.Hour)
	frame := map[string]any{"traces": []any{map[string]any{"id": owner, "endpoint": "native-task", "isTask": true, "recordedAt": recorded, "duration": 2000000,
		"spans": []any{
			map[string]any{"id": child, "name": "child", "parentSpanId": parent, "attributes": map[string]string{"native": "preserved"}, "startTime": recorded, "duration": 1000},
			map[string]any{"id": parent, "name": "parent", "parentSpanId": "", "startTime": recorded, "duration": 2000},
		}}}}
	body, err := json.Marshal(map[string]any{"serverName": "native-host", "collectionFrames": []any{frame}})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/api/report", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.ProjectIdContextKey, project)
	ClientController.Report(c)
	if rec.Code != 200 || len(c.Errors) > 0 {
		t.Fatalf("report: %d %v", rec.Code, c.Errors)
	}
	task, err := telemetry.TaskRepository.FindById(context.Background(), project, owner, &recorded)
	if err != nil || task == nil {
		t.Fatalf("task: %+v %v", task, err)
	}
	// A native run is the root span of a trace named after it, so it reads like any other.
	run := hex.EncodeToString(owner[:])
	if task.TraceId != run || task.SpanId != run || task.ParentSpanId != "" || !task.IsRoot {
		t.Fatalf("native task ids: %+v", task)
	}
	graph, err := telemetry.SpanRepository.FindGraph(context.Background(), shared.NewSpanLookup(project, task.TraceId, task.SpanId, task.RecordedAt))
	if err != nil {
		t.Fatal(err)
	}
	spans := graph.Spans
	if len(spans) != 2 {
		t.Fatalf("native graph: %+v", spans)
	}
	for _, span := range spans {
		switch span.SpanId {
		case hex.EncodeToString(child[:]):
			if span.ParentSpanId != hex.EncodeToString(parent[:]) || span.Attributes["native"] != "preserved" {
				t.Fatalf("lost native edge: %+v", span)
			}
		case hex.EncodeToString(parent[:]):
			if span.ParentSpanId != run {
				t.Fatalf("a span that names no parent hangs under the run: %+v", span)
			}
		default:
			t.Fatalf("unexpected span: %+v", span)
		}
		if !span.StartTime.Equal(recorded) || span.TraceId != run || span.ServiceName != "native-host" {
			t.Fatalf("native identity/time changed: %+v", span)
		}
	}
	whole, err := telemetry.SpanRepository.FindTrace(context.Background(), []uuid.UUID{project}, run, recorded)
	if err != nil || len(whole.Spans) != 3 {
		t.Fatalf("the whole trace is the run's own span and its two children: %+v %v", whole, err)
	}
	for _, span := range whole.Spans {
		if (span.ParentSpanId == "") != (span.Name == "native-task") {
			t.Fatalf("the run's own span is the only root: %+v", span)
		}
	}
	exported := map[string]*tracepb.Span{}
	for _, span := range whole.Spans {
		payload, err := telemetry.OtelSpanRepository.FindOTLP(context.Background(), project, run, span.SpanId, recorded)
		if err != nil {
			t.Fatal(err)
		}
		var resource tracepb.ResourceSpans
		if err := proto.Unmarshal(payload, &resource); err != nil {
			t.Fatal(err)
		}
		if len(resource.ScopeSpans) != 1 || len(resource.ScopeSpans[0].Spans) != 1 || shared.StringAttributes(resource.Resource.GetAttributes())["service.name"] != "native-host" {
			t.Fatalf("native export lost its envelope: %v", &resource)
		}
		source := resource.ScopeSpans[0].Spans[0]
		if len(source.TraceId) != 16 || len(source.SpanId) != 8 || (len(source.ParentSpanId) != 0 && len(source.ParentSpanId) != 8) || hex.EncodeToString(source.TraceId) != run {
			t.Fatalf("invalid native OTLP identity: %v", source)
		}
		if shared.StringAttributes(source.Attributes)["traceway.native.span_id"] != span.SpanId {
			t.Fatal("export must retain the native API identity")
		}
		exported[span.SpanId] = source
	}
	for _, span := range whole.Spans {
		if span.ParentSpanId != "" && !bytes.Equal(exported[span.SpanId].ParentSpanId, exported[span.ParentSpanId].SpanId) {
			t.Fatal("native parent and child use different OTLP identity mappings")
		}
	}
	if shared.StringAttributes(exported[hex.EncodeToString(child[:])].Attributes)["native"] != "preserved" {
		t.Fatal("native attributes were not exported")
	}
	// A malformed legacy ID must generate one owner for both projection and children.
	trace := &clientmodels.ClientTrace{Id: "invalid"}
	id := trace.ParsedId()
	if trace.ToTask("", "").Id != id || trace.ToEndpoint("", "").Id != id {
		t.Fatal("generated owner identity changed between conversions")
	}
}
