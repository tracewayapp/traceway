//go:build !telemetry_ch && !telemetry_duckdb && !transactional_pg

package otelcontrollers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/clientlogs"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/dbtest"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestClientLogThrottlePersistsReviewAndReturnsPartialSuccess(t *testing.T) {
	dbtest.SetupSQLite(t)
	setFakeStore(t, nil)
	previous := config.Config
	config.Config = &config.Cfg{}
	t.Cleanup(func() { config.Config = previous })
	for _, encoding := range []string{"application/json", "application/x-protobuf"} {
		t.Run(encoding, func(t *testing.T) {
			project := uuid.New()
			session := uuid.New()
			now := time.Now().UTC()
			records := make([]*logspb.LogRecord, 1003)
			for i := range records {
				records[i] = eventRecord("console.log", now, strKV(attrSessionId, session.String()), strKV("device.id", "device-a"))
			}
			req := eventLogs(browserResource(), records...)
			var payload []byte
			var err error
			if encoding == "application/json" {
				payload, err = protojson.Marshal(req)
			} else {
				payload, err = proto.Marshal(req)
			}
			if err != nil {
				t.Fatal(err)
			}
			post := func() *collogspb.ExportLogsServiceResponse {
				t.Helper()
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/api/otel/v1/logs", bytes.NewReader(payload))
				c.Request.Header.Set("Content-Type", encoding)
				c.Request.RemoteAddr = "192.0.2.55:4321"
				c.Set(middleware.ProjectIdContextKey, project)
				c.Set(middleware.ProjectContextKey, browserProject(project))
				OtelController.ExportLogs(c)
				if recorder.Code != 200 || len(c.Errors) > 0 {
					t.Fatalf("status=%d errors=%v", recorder.Code, c.Errors)
				}
				response := &collogspb.ExportLogsServiceResponse{}
				if encoding == "application/json" {
					err = protojson.Unmarshal(recorder.Body.Bytes(), response)
				} else {
					err = proto.Unmarshal(recorder.Body.Bytes(), response)
				}
				if err != nil {
					t.Fatal(err)
				}
				return response
			}
			if got := post().GetPartialSuccess().GetRejectedLogRecords(); got != 3 {
				t.Fatalf("rejected=%d", got)
			}
			if got := post().GetPartialSuccess().GetRejectedLogRecords(); got != 1003 {
				t.Fatalf("repeat rejected=%d", got)
			}
			_, total, err := telemetry.LogRecordRepository.Search(context.Background(), telemetry.LogSearchParams{ProjectId: project, FromDate: now.Add(-time.Minute), ToDate: now.Add(time.Minute), Page: 1, PageSize: 10})
			if err != nil || total != 1000 {
				t.Fatalf("stored=%d err=%v", total, err)
			}
			issues, err := telemetry.ExceptionStackTraceRepository.FindAllBySessionId(context.Background(), project, session, now)
			if err != nil || len(issues) != 1 {
				t.Fatalf("reviews=%d err=%v", len(issues), err)
			}
			if issues[0].Attributes["client.ip"] != "192.0.2.55" || issues[0].Attributes["device.id"] != "device-a" || issues[0].StackTrace != clientlogs.ReviewMessage {
				t.Fatalf("review=%+v", issues[0])
			}
		})
	}
}

func TestBackendLogsAreNotIPLimited(t *testing.T) {
	records := make([]*logspb.LogRecord, 1001)
	for i := range records {
		records[i] = &logspb.LogRecord{}
	}
	req := eventLogs(nil, records...)
	dropped, review := throttleLogs(&models.Project{Framework: "go"}, uuid.New(), "192.0.2.1", req)
	if dropped != 0 || review != nil || len(req.ResourceLogs[0].ScopeLogs[0].LogRecords) != 1001 {
		t.Fatal("backend logs were limited")
	}
}

func TestThrottlePrioritizesSessionAndExceptionEvents(t *testing.T) {
	records := make([]*logspb.LogRecord, 1000)
	for i := range records {
		records[i] = eventRecord("console.log", time.Now())
	}
	records = append(records, eventRecord(eventSessionStart, time.Now()), eventRecord(eventException, time.Now()))
	req := eventLogs(nil, records...)
	dropped, _ := throttleLogs(browserProject(uuid.New()), uuid.New(), "192.0.2.1", req)
	kept := req.ResourceLogs[0].ScopeLogs[0].LogRecords
	if dropped != 2 || len(kept) != 1000 || kept[998].EventName != eventSessionStart || kept[999].EventName != eventException {
		t.Fatal("important events were discarded before ordinary logs")
	}
}
