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
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/dbtest"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"google.golang.org/protobuf/proto"
)

func TestExportLogsWritesBrowserSessionsAndIssues(t *testing.T) {
	dbtest.SetupSQLite(t)
	setFakeStore(t, nil)
	previousConfig := config.Config
	config.Config = &config.Cfg{}
	t.Cleanup(func() { config.Config = previousConfig })
	gin.SetMode(gin.TestMode)

	sessionId := uuid.MustParse("0195f2a0-0000-7000-8000-00000000d001")
	exceptionId := uuid.MustParse("0195f2a0-0000-7000-8000-00000000d002")
	started := time.Now().UTC().Add(-5 * time.Minute).Truncate(time.Millisecond)
	sid := strKV(attrSessionId, sessionId.String())
	startedMs := intKV(attrSessionStarted, started.UnixMilli())

	req := eventLogs(browserResource(),
		eventRecord(eventSessionStart, started, sid, startedMs, strKV("user.id", "u-1")),
		eventRecord(eventException, started.Add(time.Minute), sid,
			strKV("exception.type", "Error"),
			strKV("exception.message", "boom"),
			strKV(attrTracewayExceptionId, exceptionId.String()),
		),
	)
	payload, err := proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/otel/v1/logs", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/x-protobuf")
	c.Request.RemoteAddr = "198.51.100.7:4321"
	c.Set(middleware.ProjectIdContextKey, testProjectId)
	c.Set(middleware.ProjectContextKey, browserProject(testProjectId))
	OtelController.ExportLogs(c)
	if recorder.Code != http.StatusOK || len(c.Errors) > 0 {
		t.Fatalf("export failed: %d %v", recorder.Code, c.Errors)
	}

	ctx := context.Background()
	session, err := telemetry.SessionRepository.FindById(ctx, testProjectId, sessionId, nil)
	if err != nil || session == nil {
		t.Fatalf("session not stored: %v", err)
	}
	if session.ClientIP != "198.51.100.7" || session.Attributes["user.id"] != "u-1" {
		t.Errorf("unexpected session: ip=%q attrs=%v", session.ClientIP, session.Attributes)
	}

	exceptions, err := telemetry.ExceptionStackTraceRepository.FindAllBySessionId(ctx, testProjectId, sessionId, started)
	if err != nil {
		t.Fatal(err)
	}
	if len(exceptions) != 1 || exceptions[0].Id != exceptionId {
		t.Fatalf("expected the client-identified exception under the session, got %+v", exceptions)
	}
}
