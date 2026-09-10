//go:build !telemetry_ch && !telemetry_duckdb

package clientcontrollers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
)

func TestReportSessionAttributesCompatibility(t *testing.T) {
	dbtest.SetupSQLite(t)
	gin.SetMode(gin.TestMode)
	projectID := uuid.New()
	router := gin.New()
	router.POST("/api/report", func(c *gin.Context) { c.Set(middleware.ProjectIdContextKey, projectID) }, middleware.UseGzip, ClientController.Report)
	now := time.Now().UTC().Truncate(time.Millisecond)
	post := func(t *testing.T, frame map[string]any, compressed bool) {
		t.Helper()
		payload, err := json.Marshal(map[string]any{"appVersion": "1.2.0", "collectionFrames": []any{frame}})
		if err != nil {
			t.Fatal(err)
		}
		if compressed {
			var buf bytes.Buffer
			zw := gzip.NewWriter(&buf)
			if _, err := zw.Write(payload); err != nil {
				t.Fatal(err)
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			payload = buf.Bytes()
		}
		req := httptest.NewRequest(http.MethodPost, "/api/report", bytes.NewReader(payload))
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("Content-Type", "application/json")
		if compressed {
			req.Header.Set("Content-Encoding", "gzip")
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("report: %d %s", rec.Code, rec.Body.String())
		}
	}
	t.Run("legacy frame without sessions", func(t *testing.T) {
		post(t, map[string]any{"traces": []any{}, "metrics": []any{}, "stackTraces": []any{}}, true)
	})
	for _, compressed := range []bool{false, true} {
		for _, withAttributes := range []bool{false, true} {
			id := uuid.New()
			session := map[string]any{"id": id.String(), "startedAt": now}
			if withAttributes {
				session["attributes"] = map[string]string{"userId": "u_42", "email": "alice@example.com", "client.ip": "forged"}
			}
			post(t, map[string]any{"sessions": []any{session}}, compressed)
			got, err := telemetry.SessionRepository.FindById(context.Background(), projectID, id, nil)
			if err != nil || got == nil {
				t.Fatalf("read session: %v, %v", got, err)
			}
			if got.Attributes["client.ip"] != "192.0.2.1" {
				t.Fatalf("client IP = %q", got.Attributes["client.ip"])
			}
			if withAttributes && got.Attributes["userId"] != "u_42" {
				t.Fatalf("attributes = %v", got.Attributes)
			}
			// A refresh must become searchable before a closing payload arrives.
			session["attributes"] = map[string]string{"userId": "u_43"}
			post(t, map[string]any{"sessions": []any{session}}, compressed)
			rows, _, err := telemetry.SessionRepository.FindAll(context.Background(), projectID, now.Add(-time.Hour), now.Add(time.Hour), 1, 50, "started_at", "desc", id.String(), []telemetry.SessionAttributeFilter{{Key: "userId", Value: "u_43"}})
			if err != nil || len(rows) != 1 || rows[0].EndedAt != nil {
				t.Fatalf("refreshed session: %v, %v", rows, err)
			}
			session["endedAt"] = now.Add(time.Minute)
			post(t, map[string]any{"sessions": []any{session}}, compressed)
			got, err = telemetry.SessionRepository.FindById(context.Background(), projectID, id, nil)
			if err != nil || got == nil || got.EndedAt == nil || got.Attributes["userId"] != "u_43" {
				t.Fatalf("closed session: %v, %v", got, err)
			}
		}
	}
}
