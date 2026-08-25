package clientcontrollers

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/middleware"
)

func TestReportAnswers413ForOversizedBody(t *testing.T) {
	config.Init(&config.Cfg{ReportMaxBodyMB: "1"})

	gzipBody := func(t *testing.T, payload []byte) []byte {
		t.Helper()
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err := zw.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}

	oversized := []byte(`{"appVersion":"1.0.0","serverName":"t","collectionFrames":[{"traces":[],"metrics":[],"stackTraces":[{"stackTrace":"` +
		strings.Repeat("x", 2<<20) + `","recordedAt":"2026-01-15T10:30:00Z","attributes":{},"isMessage":false,"isTask":false}]}]}`)

	cases := []struct {
		name     string
		body     []byte
		encoding string
		want     int
	}{
		{"oversized gzipped body", gzipBody(t, oversized), "gzip", http.StatusRequestEntityTooLarge},
		{"oversized plain body", oversized, "", http.StatusRequestEntityTooLarge},
		{"malformed json is still a 400", []byte(`{"nope"`), "", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.POST("/api/report",
				func(c *gin.Context) { c.Set(middleware.ProjectIdContextKey, uuid.New()) },
				middleware.UseGzip,
				ClientController.Report,
			)

			req := httptest.NewRequest(http.MethodPost, "/api/report", bytes.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.encoding != "" {
				req.Header.Set("Content-Encoding", tc.encoding)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("got %d %s, want %d", rec.Code, rec.Body.String(), tc.want)
			}
		})
	}
}
