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

// An exception row says which trace and span it happened on and nothing else. A detail page shows the ones on its own
// span or below it, whichever arrived first, and the issue page finds its way back the same way.
func TestDetailShowsTheExceptionsInsideItsSubtree(t *testing.T) {
	setupSetupControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	user, org := createSetupTestAccount(t, tx, "exceptions@example.com", "owner")
	project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "Exceptions", "opentelemetry", org)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	ctx, trace, now := context.Background(), uuid.New(), time.Now().UTC()
	rootID := uuid.MustParse("00000000-0000-0000-0102-030405060708")
	childID := uuid.MustParse("00000000-0000-0000-1112-131415161718")
	outsideID := uuid.MustParse("00000000-0000-0000-2122-232425262728")
	traceHex := hex.EncodeToString(trace[:])
	spanHex := func(id uuid.UUID) string { return hex.EncodeToString(id[8:]) }
	owner := uuid.New()

	span := func(id uuid.UUID, parent *uuid.UUID) models.OtelSpan {
		stored := models.OtelSpan{Span: models.Span{ProjectId: project.Id, TraceId: traceHex, SpanId: spanHex(id), Name: "operation", StartTime: now, Duration: time.Millisecond}}
		if parent != nil {
			stored.ParentSpanId = spanHex(*parent)
		}
		return stored
	}
	if _, err := telemetry.OtelSpanRepository.InsertAsync(ctx, []models.OtelSpan{span(rootID, nil), span(childID, &rootID), span(outsideID, nil)}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{{Id: owner, ProjectId: project.Id, Endpoint: "GET /exceptions", RecordedAt: now, TraceId: traceHex, SpanId: spanHex(rootID)}}); err != nil {
		t.Fatal(err)
	}
	if err := telemetry.TaskRepository.InsertAsync(ctx, []models.Task{{Id: owner, ProjectId: project.Id, TaskName: "exceptions", RecordedAt: now, TraceId: traceHex, SpanId: spanHex(rootID)}}); err != nil {
		t.Fatal(err)
	}
	unowned := func(hash string, spanID uuid.UUID, recordedAt time.Time) models.ExceptionStackTrace {
		return models.ExceptionStackTrace{Id: uuid.New(), ProjectId: project.Id, TraceId: traceHex, SpanId: spanHex(spanID), ExceptionHash: hash, StackTrace: "RuntimeError: " + hash, RecordedAt: recordedAt}
	}
	// The exception outside the subtree is older, so only the span match can keep it off the page.
	if err := telemetry.ExceptionStackTraceRepository.InsertAsync(ctx, []models.ExceptionStackTrace{unowned("outside", outsideID, now.Add(-time.Second)), unowned("inside", childID, now)}); err != nil {
		t.Fatal(err)
	}

	for _, route := range []struct {
		param   string
		handler gin.HandlerFunc
	}{
		{"endpointId", EndpointDetailController.GetEndpointDetail},
		{"taskId", TaskDetailController.GetTaskDetail},
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
				Exception *EndpointExceptionInfo `json:"exception"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			if detail.Exception == nil || detail.Exception.ExceptionHash != "inside" {
				t.Fatalf("expected the exception recorded inside the subtree, got %+v", detail.Exception)
			}
		})
	}

	for hash, related := range map[string]bool{"inside": true, "outside": false} {
		c, response := newControllerTestContext(t, nil, user, http.MethodPost, "/exception", `{"pagination":{"page":1,"pageSize":20}}`)
		c.Set(middleware.ProjectIdContextKey, project.Id)
		c.Params = gin.Params{{Key: "hash", Value: hash}}
		ExceptionStackTraceController.FindByHash(c)
		var issue struct {
			RelatedEntity *telemetry.ExceptionOwner `json:"relatedEntity"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &issue); err != nil || response.Code != 200 {
			t.Fatalf("%s: HTTP %d %v", hash, response.Code, err)
		}
		if !related {
			if issue.RelatedEntity != nil {
				t.Fatalf("an exception on a span outside every endpoint names none: %+v", issue.RelatedEntity)
			}
			continue
		}
		if issue.RelatedEntity == nil || issue.RelatedEntity.Id != owner || issue.RelatedEntity.TraceType != "endpoint" || issue.RelatedEntity.Name != "GET /exceptions" {
			t.Fatalf("the issue page finds the endpoint above the exception's span: %+v", issue.RelatedEntity)
		}
	}

	t.Run("missing descendant exception keeps partial status", func(t *testing.T) {
		previous := shared.MaxOtelGraphRows
		shared.MaxOtelGraphRows = 1
		t.Cleanup(func() { shared.MaxOtelGraphRows = previous })
		for _, route := range []struct {
			param   string
			handler gin.HandlerFunc
		}{
			{"endpointId", EndpointDetailController.GetEndpointDetail},
			{"taskId", TaskDetailController.GetTaskDetail},
		} {
			c, response := newControllerTestContext(t, nil, user, http.MethodPost, "/detail", "{}")
			c.Set(middleware.ProjectIdContextKey, project.Id)
			c.Params = gin.Params{{Key: route.param, Value: owner.String()}}
			route.handler(c)
			var detail struct {
				Exception *EndpointExceptionInfo  `json:"exception"`
				Status    *models.SpanGraphStatus `json:"spanGraphStatus"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil || response.Code != 200 {
				t.Fatalf("%s: HTTP %d %v", route.param, response.Code, err)
			}
			if detail.Exception != nil || detail.Status == nil || detail.Status.State != models.SpanGraphPartial {
				t.Fatalf("missing exceptions must not imply a complete graph: %+v", detail)
			}
		}
	})
}
