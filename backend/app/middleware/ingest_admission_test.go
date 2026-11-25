package middleware

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newAdmissionRouter(capacity int, wait time.Duration, release chan struct{}) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ingest", newIngestAdmission(capacity, wait), func(c *gin.Context) {
		if release != nil {
			<-release
		}
		c.Status(http.StatusOK)
	})
	return r
}

func TestIngestAdmissionPassesUnderCapacity(t *testing.T) {
	r := newAdmissionRouter(2, 50*time.Millisecond, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ingest", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 under capacity, got %d", w.Code)
	}
}

func TestIngestAdmissionRejectsWhenSaturated(t *testing.T) {
	release := make(chan struct{})
	r := newAdmissionRouter(1, 50*time.Millisecond, release)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ingest", nil))
	}()

	// Give the goroutine time to acquire the only slot; its handler then
	// blocks on the release channel, guaranteeing saturation below.
	time.Sleep(20 * time.Millisecond)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ingest", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when saturated, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header on 503")
	}

	close(release)
	wg.Wait()

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/ingest", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 after release, got %d", w2.Code)
	}
}

func TestIngestAdmissionWaiterGetsSlotWhenFreed(t *testing.T) {
	release := make(chan struct{})
	r := newAdmissionRouter(1, 2*time.Second, release)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ingest", nil))
	}()
	time.Sleep(20 * time.Millisecond)

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(release)
	}()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ingest", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected waiting request to get the freed slot, got %d", w.Code)
	}
	wg.Wait()
}

func TestIngestCapacityFollowsSlotsPerCPUUnlessSetExplicitly(t *testing.T) {
	previous := IngestSlotsPerCPU
	t.Cleanup(func() { IngestSlotsPerCPU = previous })

	IngestSlotsPerCPU = 12
	t.Setenv("INGEST_MAX_CONCURRENT", "")
	if got, want := ingestCapacityFromEnv(), runtime.NumCPU()*12; got != want {
		t.Fatalf("capacity = %d, want %d", got, want)
	}
	t.Setenv("INGEST_MAX_CONCURRENT", "7")
	if got := ingestCapacityFromEnv(); got != 7 {
		t.Fatalf("INGEST_MAX_CONCURRENT must win over the per-CPU default, got %d", got)
	}
}
