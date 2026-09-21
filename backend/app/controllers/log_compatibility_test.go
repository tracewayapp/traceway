//go:build !telemetry_ch && !telemetry_duckdb && !transactional_pg

package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
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
