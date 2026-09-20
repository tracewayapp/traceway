//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func TestTraceResponsesExposePartialGraphs(t *testing.T) {
	setupSetupControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	user, org := createSetupTestAccount(t, tx, "partial@example.com", "owner")
	project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Graph", "opentelemetry", org)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	ctx, trace, now := context.Background(), uuid.New(), time.Now().UTC()
	rootID := uuid.MustParse("00000000-0000-0000-0102-030405060708")
	owner, traceId, rootSpanId := uuid.New(), hex.EncodeToString(trace[:]), hex.EncodeToString(rootID[8:])
	spans := []models.OtelSpan{}
	for i, id := range []uuid.UUID{rootID, uuid.New(), uuid.New()} {
		span := models.OtelSpan{Span: models.Span{ProjectId: project.Id, TraceId: traceId, SpanId: hex.EncodeToString(id[8:]), Name: "operation", StartTime: now.Add(time.Duration(i)), Duration: time.Millisecond}}
		if i > 0 {
			span.ParentSpanId = rootSpanId
		}
		spans = append(spans, span)
	}
	if _, err := telemetry.OtelSpanRepository.InsertAsync(ctx, spans); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{{Id: owner, ProjectId: project.Id, Endpoint: "GET /graph", RecordedAt: now, TraceId: traceId, SpanId: rootSpanId}}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.TaskRepository.InsertAsync(ctx, []models.Task{{Id: owner, ProjectId: project.Id, TaskName: "graph", RecordedAt: now, TraceId: traceId, SpanId: rootSpanId}}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.AiTraceRepository.InsertAsync(ctx, []models.AiTrace{{Id: owner, ProjectId: project.Id, TraceName: "graph", RecordedAt: now, TraceId: traceId, SpanId: rootSpanId}}); err != nil {
		t.Fatal(err)
	}
	previous := shared.MaxOtelGraphRows
	shared.MaxOtelGraphRows = 2
	t.Cleanup(func() { shared.MaxOtelGraphRows = previous })
	check := func(spans []models.Span, status *models.SpanGraphStatus) {
		t.Helper()
		if len(spans) != 1 || status == nil || status.State != models.SpanGraphPartial || len(status.Reasons) != 1 || status.Reasons[0] != models.SpanGraphRowLimit {
			t.Fatalf("available spans or completeness lost: %d spans, %+v", len(spans), status)
		}
	}
	for _, route := range []struct {
		param   string
		handler gin.HandlerFunc
	}{
		{"endpointId", EndpointDetailController.GetEndpointDetail},
		{"taskId", TaskDetailController.GetTaskDetail},
		{"traceId", AiTraceController.GetAiTraceDetail},
	} {
		t.Run(route.param, func(t *testing.T) {
			c, response := newControllerTestContext(t, nil, user, http.MethodPost, "/detail", "{}")
			c.Set(middleware.ProjectIdContextKey, project.Id)
			c.Params = gin.Params{{Key: route.param, Value: owner.String()}}
			route.handler(c)
			if response.Code != 200 {
				t.Fatalf("HTTP %d: %v", response.Code, c.Errors)
			}
			var detail struct {
				Spans  []models.Span           `json:"spans"`
				Status *models.SpanGraphStatus `json:"spanGraphStatus"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			check(detail.Spans, detail.Status)
		})
	}
	c, response := newControllerTestContext(t, nil, user, http.MethodPost, "/distributed", "{}")
	c.Params = gin.Params{{Key: "traceId", Value: traceId}}
	DistributedTraceController.GetDistributedTrace(c)
	if response.Code != 200 {
		t.Fatalf("HTTP %d: %v", response.Code, c.Errors)
	}
	var distributed DistributedTraceResponse
	if err := json.Unmarshal(response.Body.Bytes(), &distributed); err != nil {
		t.Fatal(err)
	}
	if len(distributed.Nodes) != 3 {
		t.Fatalf("expected three occurrences, got %d", len(distributed.Nodes))
	}
	for _, node := range distributed.Nodes {
		check(node.Spans, node.SpanGraphStatus)
	}
}
