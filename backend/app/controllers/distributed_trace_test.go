//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func TestGetDistributedTraceAttachesSpansToOwningNodes(t *testing.T) {
	setupSetupControllerDB(t)

	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	userId, orgId := createSetupTestAccount(t, tx, "trace@example.com", "owner")
	project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Trace", "opentelemetry", orgId)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	// The handler opens its own transaction on the single-connection main DB.
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	ctx := context.Background()
	distributedTraceId := uuid.New()
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)

	endpoint := models.Endpoint{
		Id: uuid.New(), ProjectId: project.Id, Endpoint: "GET /orders", Duration: 100 * time.Millisecond,
		RecordedAt: at, StatusCode: 200, Attributes: map[string]string{}, DistributedTraceId: &distributedTraceId, IsRoot: true,
	}
	spanless := models.Endpoint{
		Id: uuid.New(), ProjectId: project.Id, Endpoint: "GET /health", Duration: time.Millisecond,
		RecordedAt: at, StatusCode: 200, Attributes: map[string]string{}, DistributedTraceId: &distributedTraceId,
	}
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{endpoint, spanless}); err != nil {
		t.Fatalf("insert endpoints: %v", err)
	}
	task := models.Task{
		Id: uuid.New(), ProjectId: project.Id, TaskName: "send-receipt", Duration: 50 * time.Millisecond,
		RecordedAt: at.Add(time.Second), Attributes: map[string]string{}, DistributedTraceId: &distributedTraceId,
	}
	if err := telemetry.TaskRepository.InsertAsync(ctx, []models.Task{task}); err != nil {
		t.Fatalf("insert task: %v", err)
	}

	span := func(traceId uuid.UUID, name string, offset time.Duration) models.Span {
		return models.Span{
			Id: uuid.New(), TraceId: traceId, ProjectId: project.Id, Name: name,
			StartTime: at.Add(offset), Duration: 5 * time.Millisecond, RecordedAt: at.Add(offset),
		}
	}
	if err := telemetry.SpanRepository.InsertAsync(ctx, []models.Span{
		span(endpoint.Id, "db.select orders", 20*time.Millisecond),
		span(endpoint.Id, "publish receipt", 10*time.Millisecond),
		span(task.Id, "smtp.send", time.Second+5*time.Millisecond),
		// An orphan span keyed by the OTel trace id belongs to no node.
		span(distributedTraceId, "orphan", 0),
	}); err != nil {
		t.Fatalf("insert spans: %v", err)
	}

	body := `{"recordedAt":"` + at.Format(time.RFC3339) + `"}`
	c, recorder := newControllerTestContext(t, nil, userId, http.MethodPost, "/api/distributed-traces/"+distributedTraceId.String(), body)
	c.Params = gin.Params{{Key: "distributedTraceId", Value: distributedTraceId.String()}}
	DistributedTraceController.GetDistributedTrace(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var resp DistributedTraceResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3: %s", len(resp.Nodes), recorder.Body.String())
	}

	spanNames := func(node DistributedTraceNode) []string {
		names := make([]string, 0, len(node.Spans))
		for _, s := range node.Spans {
			names = append(names, s.Name)
		}
		return names
	}
	for _, node := range resp.Nodes {
		switch {
		case node.Endpoint != nil && node.Endpoint.Id == endpoint.Id:
			if got := spanNames(node); strings.Join(got, ",") != "publish receipt,db.select orders" {
				t.Errorf("endpoint spans = %v, want both owned spans ordered by start time", got)
			}
		case node.Endpoint != nil && node.Endpoint.Id == spanless.Id:
			if node.Spans == nil || len(node.Spans) != 0 {
				t.Errorf("spanless endpoint spans = %v, want an empty list", node.Spans)
			}
		case node.Task != nil && node.Task.Id == task.Id:
			if got := spanNames(node); strings.Join(got, ",") != "smtp.send" {
				t.Errorf("task spans = %v, want the task's own span", got)
			}
		default:
			t.Errorf("unexpected node %+v", node)
		}
	}
	if !strings.Contains(recorder.Body.String(), `"spans":[]`) {
		t.Errorf("a node without spans must serialize an empty array, got %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), `"orphan"`) {
		t.Errorf("the orphan span was attached to a node: %s", recorder.Body.String())
	}
}
