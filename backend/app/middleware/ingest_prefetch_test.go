package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// prefetchRouter puts the read-ahead in front of a one-slot gate, as the ingest
// routes do, and reports every body its handler read.
func prefetchRouter(budget *byteBudget, maxBytes int64, bodies chan<- string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	prefetch := func(c *gin.Context) { prefetchIngestBody(c, budget, DefaultBodyIdle, DefaultBodyTotal, maxBytes) }
	r.POST("/x", prefetch, newAdmissionGate("test", 1, 50*time.Millisecond, "busy", nil, "2"), func(c *gin.Context) {
		data, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		bodies <- string(data)
		c.Status(http.StatusOK)
	})
	return r
}

func declaredRequest(body io.Reader, length int64) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/x", body)
	req.ContentLength = length
	return req
}

func serveAsync(r *gin.Engine, req *http.Request) <-chan int {
	done := make(chan int, 1)
	go func() {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		done <- rec.Code
	}()
	return done
}

func TestPrefetchKeepsASlowUploadOutOfTheIngestSlot(t *testing.T) {
	budget := &byteBudget{limit: 1 << 20}
	bodies := make(chan string, 2)
	r := prefetchRouter(budget, 1<<20, bodies)

	slowBody, slowClient := io.Pipe()
	slow := serveAsync(r, declaredRequest(slowBody, 5))
	if _, err := slowClient.Write([]byte("sl")); err != nil {
		t.Fatal(err)
	}

	// The gate has one slot and waits 50ms; a slow upload holding it would turn this away.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, declaredRequest(strings.NewReader("quick"), 5))
	if rec.Code != http.StatusOK {
		t.Fatalf("a request behind a slow upload got %d, want 200", rec.Code)
	}
	if got := <-bodies; got != "quick" {
		t.Fatalf("handler read %q", got)
	}

	if _, err := slowClient.Write([]byte("owl")); err != nil {
		t.Fatal(err)
	}
	slowClient.Close()
	if code := <-slow; code != http.StatusOK {
		t.Fatalf("the slow upload got %d, want 200", code)
	}
	if got := <-bodies; got != "slowl" {
		t.Fatalf("handler read %q, want the whole slow body", got)
	}
	if used := budget.used.Load(); used != 0 {
		t.Fatalf("budget still holds %d bytes after both requests", used)
	}
}

func TestPrefetchLeavesBodiesItCannotTakeToTheHandler(t *testing.T) {
	cases := []struct {
		name   string
		budget *byteBudget
		max    int64
		length int64
	}{
		{"no declared length", &byteBudget{limit: 1 << 20}, 1 << 20, -1},
		{"over the route cap", &byteBudget{limit: 1 << 20}, 4, 7},
		{"over its share of the budget", &byteBudget{limit: 16}, 1 << 20, 7},
		{"budget spent", func() *byteBudget { b := &byteBudget{limit: 64}; b.used.Store(60); return b }(), 1 << 20, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bodies := make(chan string, 1)
			r := prefetchRouter(tc.budget, tc.max, bodies)
			used := tc.budget.used.Load()
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, declaredRequest(bytes.NewReader([]byte("payload")), tc.length))
			if rec.Code != http.StatusOK {
				t.Fatalf("got %d, want 200", rec.Code)
			}
			if got := <-bodies; got != "payload" {
				t.Fatalf("handler read %q", got)
			}
			if tc.budget.used.Load() != used {
				t.Fatalf("budget changed from %d to %d", used, tc.budget.used.Load())
			}
		})
	}
}

func TestPrefetchRejectsATruncatedBodyAndFreesTheBudget(t *testing.T) {
	budget := &byteBudget{limit: 1 << 20}
	bodies := make(chan string, 1)
	r := prefetchRouter(budget, 1<<20, bodies)

	body, client := io.Pipe()
	done := serveAsync(r, declaredRequest(body, 10))
	if _, err := client.Write([]byte("part")); err != nil {
		t.Fatal(err)
	}
	client.CloseWithError(io.ErrUnexpectedEOF)

	if code := <-done; code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 for a body the client abandoned", code)
	}
	select {
	case got := <-bodies:
		t.Fatalf("the handler ran on a truncated body: %q", got)
	default:
	}
	if used := budget.used.Load(); used != 0 {
		t.Fatalf("budget still holds %d bytes", used)
	}
}

func TestByteBudgetNeverOvercommits(t *testing.T) {
	b := &byteBudget{limit: 10}
	if !b.tryAcquire(6) || b.tryAcquire(5) || !b.tryAcquire(4) {
		t.Fatal("budget admitted the wrong amounts")
	}
	b.release(6)
	if !b.tryAcquire(6) {
		t.Fatal("released bytes were not returned to the budget")
	}
}
