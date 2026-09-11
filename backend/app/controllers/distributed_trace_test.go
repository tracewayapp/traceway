//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func TestGetDistributedTraceIncludesOwnedSpans(t *testing.T) {
	setupSetupControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	userID, orgID := createSetupTestAccount(t, tx, "distributed@example.com", "owner")
	_, privateOrgID := createSetupTestAccount(t, tx, "private@example.com", "owner")
	createProject := func(name string, org int) uuid.UUID {
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, name, "opentelemetry", org)
		if err != nil {
			t.Fatal(err)
		}
		return project.Id
	}
	project1, project2 := createProject("API", orgID), createProject("Worker", orgID)
	privateProject := createProject("Private", privateOrgID)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	later := now.Add(36 * time.Hour)
	distributedID := uuid.New()
	occurrenceID := uuid.MustParse("00000000-0000-0000-1985-a7abed0024db")
	aiID, emptyID, orphanID := uuid.New(), uuid.New(), uuid.New()
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{{
		Id: occurrenceID, ProjectId: project1, Endpoint: "GET /jobs", RecordedAt: now, DistributedTraceId: &distributedID,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.TaskRepository.InsertAsync(ctx, []models.Task{
		{Id: occurrenceID, ProjectId: project2, TaskName: "worker", RecordedAt: later, DistributedTraceId: &distributedID},
		{Id: emptyID, ProjectId: project1, TaskName: "empty", RecordedAt: now, DistributedTraceId: &distributedID},
		{Id: occurrenceID, ProjectId: privateProject, TaskName: "private", RecordedAt: now, DistributedTraceId: &distributedID},
	}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.AiTraceRepository.InsertAsync(ctx, []models.AiTrace{{
		Id: aiID, ProjectId: project1, TraceName: "chat", RecordedAt: now, DistributedTraceId: &distributedID,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{
		{Id: uuid.New(), ProjectId: project2, TraceId: &occurrenceID, ExceptionHash: "worker-error", RecordedAt: later, DistributedTraceId: &distributedID},
		{Id: uuid.New(), ProjectId: project2, TraceId: &orphanID, ExceptionHash: "orphan", RecordedAt: now, DistributedTraceId: &distributedID},
		{Id: uuid.New(), ProjectId: project1, ExceptionHash: "no-trace", RecordedAt: now, DistributedTraceId: &distributedID},
	}); err != nil {
		t.Fatal(err)
	}
	makeSpan := func(project, owner uuid.UUID, name string, at time.Time) models.Span {
		return models.Span{Id: uuid.New(), ProjectId: project, TraceId: owner, Name: name, StartTime: at, RecordedAt: at, Duration: time.Millisecond}
	}
	parent := uuid.New()
	first := makeSpan(project1, occurrenceID, "first", now)
	first.ParentSpanId = &parent
	first.Attributes = map[string]string{"db.system": "postgresql"}
	if err := telemetry.SpanRepository.InsertAsync(ctx, []models.Span{
		makeSpan(project1, occurrenceID, "second", now.Add(time.Second)), first,
		makeSpan(project2, occurrenceID, "worker-child", later),
		makeSpan(project1, aiID, "ai-child", now),
		makeSpan(project2, orphanID, "orphan-child", now),
		makeSpan(privateProject, occurrenceID, "private-child", now),
		makeSpan(project1, occurrenceID, "outside-window", now.Add(-25*time.Hour)),
		makeSpan(project1, distributedID, "distributed-id-is-not-owner", now),
	}); err != nil {
		t.Fatal(err)
	}

	for _, body := range []string{"{}", `{"recordedAt":"` + now.Format(time.RFC3339Nano) + `"}`} {
		c, recorder := newControllerTestContext(t, nil, userID, http.MethodPost, "/distributed-traces/"+distributedID.String(), body)
		c.Params = gin.Params{{Key: "distributedTraceId", Value: distributedID.String()}}
		DistributedTraceController.GetDistributedTrace(c)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status %d: %s; errors: %v", recorder.Code, recorder.Body.String(), c.Errors)
		}
		var response DistributedTraceResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Nodes) != 6 {
			t.Fatalf("got %d nodes, want 6: %s", len(response.Nodes), recorder.Body.String())
		}
		for _, node := range response.Nodes {
			if node.Spans == nil {
				t.Fatal("spans must serialize as an array")
			}
			switch {
			case node.Endpoint != nil:
				if len(node.Spans) != 2 || node.Spans[0].Name != "first" || node.Spans[1].Name != "second" {
					t.Fatalf("endpoint spans: %+v", node.Spans)
				}
				if node.Exception != nil || node.Spans[0].ParentSpanId == nil || *node.Spans[0].ParentSpanId != parent || node.Spans[0].Attributes["db.system"] != "postgresql" {
					t.Fatalf("incorrect endpoint metadata: %+v", node)
				}
			case node.Task != nil && node.Task.Id == occurrenceID:
				if node.ProjectId != project2 || len(node.Spans) != 1 || node.Spans[0].Name != "worker-child" || node.Exception == nil || node.Exception.ExceptionHash != "worker-error" {
					t.Fatalf("worker node: %+v", node)
				}
			case node.AiTrace != nil:
				if len(node.Spans) != 1 || node.Spans[0].Name != "ai-child" {
					t.Fatalf("AI spans: %+v", node.Spans)
				}
			case node.Exception != nil && node.Exception.ExceptionHash == "orphan":
				if len(node.Spans) != 1 || node.Spans[0].Name != "orphan-child" {
					t.Fatalf("orphan spans: %+v", node.Spans)
				}
			default:
				if len(node.Spans) != 0 {
					t.Fatalf("expected no spans: %+v", node)
				}
			}
		}
	}

	if _, err := db.TelemetryDB.Exec("DROP TABLE spans"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uuid.UUID{uuid.New(), distributedID} {
		c, recorder := newControllerTestContext(t, nil, userID, http.MethodPost, "/distributed-traces/"+id.String(), "{}")
		c.Params = gin.Params{{Key: "distributedTraceId", Value: id.String()}}
		DistributedTraceController.GetDistributedTrace(c)
		if id == distributedID {
			if recorder.Code != http.StatusInternalServerError || len(c.Errors) == 0 {
				t.Fatalf("span lookup failure must return 500, got %d", recorder.Code)
			}
		} else {
			var response DistributedTraceResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != http.StatusOK || response.Nodes == nil || len(response.Nodes) != 0 {
				t.Fatalf("empty trace should skip span queries and return [], got %s", recorder.Body.String())
			}
		}
	}
}
