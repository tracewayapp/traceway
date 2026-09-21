//go:build !telemetry_ch && !telemetry_duckdb && !transactional_pg

package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func TestLogTraceFiltersStayScoped(t *testing.T) {
	dbtest.SetupSQLite(t)
	ctx := context.Background()
	project, other, browser, server := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	hexID := func(id uuid.UUID) string { return strings.ReplaceAll(id.String(), "-", "") }
	at := time.Now().UTC().Truncate(time.Second)
	if err := telemetry.EndpointRepository.InsertAsync(ctx, []models.Endpoint{{Id: uuid.New(), ProjectId: project, TraceId: hexID(server), Attributes: map[string]string{"traceway.distributed_trace_id": hexID(browser)}, SpanId: "0102030405060708", RecordedAt: at}}); err != nil {
		t.Fatal(err)
	}
	logs := []models.LogRecord{
		{Id: uuid.New(), ProjectId: project, TraceId: hexID(browser), Timestamp: at, Body: "browser"},
		{Id: uuid.New(), ProjectId: project, TraceId: hexID(server), Timestamp: at, Body: "server"},
		{Id: uuid.New(), ProjectId: project, TraceId: hexID(uuid.New()), Timestamp: at, Body: "unrelated"},
		{Id: uuid.New(), ProjectId: other, TraceId: hexID(server), Timestamp: at, Body: "other project"},
	}
	if err := telemetry.LogRecordRepository.InsertAsync(ctx, logs); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		trace, span, retired string
		status, want         int
	}{
		{browser.String(), "", "", 200, 1}, {hexID(server), "", "", 200, 1}, {"invalid", "", "", 400, 0}, {strings.Repeat("0", 32), "", "", 400, 0}, {"", "", "distributedTraceId", 400, 0}, {"", "", "excludeTraceId", 400, 0},
	} {
		payload := map[string]any{"fromDate": at.Add(-time.Hour), "toDate": at.Add(time.Hour), "traceId": test.trace, "spanId": test.span, "pagination": map[string]int{"page": 1, "pageSize": 100}}
		if test.retired != "" {
			payload[test.retired] = hexID(browser)
		}
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest("POST", "/api/logs", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(middleware.ProjectIdContextKey, project)
		LogController.List(c)
		if rec.Code != test.status {
			t.Fatalf("got %d, want %d: %s", rec.Code, test.status, rec.Body.String())
		}
		if test.status == 200 {
			var result PaginatedResponse[models.LogRecord]
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Data) != test.want || result.Pagination.Total != int64(test.want) {
				t.Fatalf("trace filter widened or lost results: %+v", result)
			}
			for _, log := range result.Data {
				if log.ProjectId != project || log.Body == "unrelated" {
					t.Fatal("trace filter escaped its scope")
				}
			}
		}
	}
}

func TestWholeTraceLogsStayInsideSelectedOrganization(t *testing.T) {
	setupSetupControllerDB(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	user, org := createSetupTestAccount(t, tx, "trace-logs@example.com", "owner")
	_, otherOrg := createSetupTestAccount(t, tx, "other-trace-logs@example.com", "owner")
	_, strangerOrg := createSetupTestAccount(t, tx, "stranger-trace-logs@example.com", "owner")
	if _, err := transactional.OrganizationRepository.AddUser(tx, otherOrg, user, "user"); err != nil {
		t.Fatal(err)
	}
	var projects []*models.Project
	for i, organization := range []int{org, org, otherOrg, strangerOrg} {
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, fmt.Sprintf("service-%d", i), "opentelemetry", organization)
		if err != nil {
			t.Fatal(err)
		}
		projects = append(projects, project)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	const trace = "0123456789abcdef0123456789abcdef"
	at := time.Now().UTC().Truncate(time.Second)
	for _, project := range projects {
		if err := telemetry.LogRecordRepository.InsertAsync(context.Background(), []models.LogRecord{{
			Id: uuid.New(), ProjectId: project.Id, TraceId: trace, Timestamp: at, Body: project.Name,
		}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, selected := range projects[:2] {
		for _, whole := range []bool{false, true} {

			body := fmt.Sprintf(`{"traceId":%q,"wholeTrace":%t,"fromDate":%q,"toDate":%q,"pagination":{"page":1,"pageSize":100}}`, trace, whole, at.Add(-time.Hour).Format(time.RFC3339Nano), at.Add(time.Hour).Format(time.RFC3339Nano))
			c, response := newControllerTestContext(t, nil, user, "POST", "/logs", body)
			c.Set(middleware.ProjectIdContextKey, selected.Id)
			LogController.List(c)
			var result PaginatedResponse[models.LogRecord]
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 {
				t.Fatalf("%d %s %v", response.Code, response.Body.String(), err)
			}
			want := 1
			if whole {
				want = 2
			}
			if len(result.Data) != want || result.Pagination.Total != int64(want) {
				t.Fatalf("whole=%t: %+v", whole, result)
			}
			for _, log := range result.Data {
				if log.ProjectId != projects[0].Id && log.ProjectId != projects[1].Id {
					t.Fatal("whole trace leaked another organization's log")
				}
			}
		}
	}
	c, response := newControllerTestContext(t, nil, user, "POST", "/logs", `{"wholeTrace":true,"pagination":{"page":1,"pageSize":100}}`)
	c.Set(middleware.ProjectIdContextKey, projects[0].Id)
	LogController.List(c)
	if response.Code != 400 {
		t.Fatalf("wholeTrace without traceId must be rejected: %d", response.Code)
	}
}
