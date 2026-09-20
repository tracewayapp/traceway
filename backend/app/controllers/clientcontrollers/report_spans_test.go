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
	// A malformed legacy ID must generate one owner for both projection and children.
	trace := &clientmodels.ClientTrace{Id: "invalid"}
	id := trace.ParsedId()
	if trace.ToTask("", "").Id != id || trace.ToEndpoint("", "").Id != id {
		t.Fatal("generated owner identity changed between conversions")
	}
}
