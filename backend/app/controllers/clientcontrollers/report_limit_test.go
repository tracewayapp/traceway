package clientcontrollers

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
)

func TestReportOversizedBody(t *testing.T) {
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

	// More than the cap is still unread when the limit trips, so the rest
	// cannot be taken off the wire and the request is turned away.
	farOversized := []byte(`{"appVersion":"` + strings.Repeat("x", 4<<20) + `"}`)

	cases := []struct {
		name          string
		body          []byte
		encoding      string
		want          int
		wantDiscarded bool
	}{
		{"oversized gzipped body is discarded with a 200 so the SDK drops the batch", gzipBody(t, oversized), "gzip", http.StatusOK, true},
		{"oversized plain body whose rest fits the cap is discarded", oversized[:(1<<20)+(1<<19)], "", http.StatusOK, true},
		{"body far over the cap on the wire stays a 413", farOversized, "", http.StatusRequestEntityTooLarge, false},
		{"malformed json is still a 400", []byte(`{"nope"`), "", http.StatusBadRequest, false},
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
			if got := rec.Header().Get("X-Traceway-Discarded") == "body_too_large"; got != tc.wantDiscarded {
				t.Fatalf("discarded header present=%v, want %v", got, tc.wantDiscarded)
			}
		})
	}
}

// The shipped browser SDK only lets go of a batch on a 200 it actually
// receives, so the answer has to arrive over a real connection while the
// client is still uploading, not as a reset.
func TestReportOversizedBodyAnswerReachesAStreamingClient(t *testing.T) {
	config.Init(&config.Cfg{ReportMaxBodyMB: "1"})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/report",
		func(c *gin.Context) { c.Set(middleware.ProjectIdContextKey, uuid.New()) },
		middleware.UseGzip,
		ClientController.Report,
	)
	server := httptest.NewServer(router)
	defer server.Close()

	// Octal digits compress about 2.7:1: under the cap on the wire, over it decoded.
	digits := make([]byte, 2<<20)
	state := uint64(0x9E3779B97F4A7C15)
	for i := range digits {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		digits[i] = '0' + byte(state&7)
	}
	var wire bytes.Buffer
	zw := gzip.NewWriter(&wire)
	if _, err := zw.Write([]byte(`{"appVersion":"` + string(digits) + `"}`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if wire.Len() >= 1<<20 || wire.Len() < 1<<19 {
		t.Fatalf("fixture is %d bytes on the wire, want between 512KB and 1MB", wire.Len())
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/report", &slowReader{data: wire.Bytes(), chunk: 16 << 10})
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Encoding", "gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("the client saw a transport error instead of a response: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Traceway-Discarded") != "body_too_large" {
		t.Fatalf("got %d (discarded=%q), want a 200 that tells the SDK to drop the batch", resp.StatusCode, resp.Header.Get("X-Traceway-Discarded"))
	}
}

type slowReader struct {
	data  []byte
	chunk int
}

func (r *slowReader) Read(b []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	time.Sleep(time.Millisecond)
	n := copy(b, r.data[:min(r.chunk, len(r.data))])
	r.data = r.data[n:]
	return n, nil
}
