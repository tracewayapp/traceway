package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func admissionRouter(gate gin.HandlerFunc, release chan struct{}) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", gate, func(c *gin.Context) {
		if release != nil {
			<-release
		}
		c.Status(http.StatusOK)
	})
	return r
}

func post(r *gin.Engine) int {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", nil))
	return rec.Code
}

func TestAdmissionGateAdmitsUpToCapacity(t *testing.T) {
	release := make(chan struct{})
	r := admissionRouter(newAdmissionGate(2, 10*time.Millisecond, "busy", nil), release)

	var wg sync.WaitGroup
	codes := make([]int, 3)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = post(r)
		}(i)
	}
	time.Sleep(150 * time.Millisecond)
	close(release)
	wg.Wait()

	ok, rejected := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusServiceUnavailable:
			rejected++
		}
	}
	if ok != 2 || rejected != 1 {
		t.Fatalf("capacity 2 admitted %d and rejected %d, want 2 and 1 (codes %v)", ok, rejected, codes)
	}
}

func TestAdmissionGateBoundsWaiters(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	r := admissionRouter(newAdmissionGate(1, time.Second, "busy", nil), release)

	codes := make(chan int, 32)
	for i := 0; i < 1+16+3; i++ {
		go func() { codes <- post(r) }()
	}

	immediate := 0
	timeout := time.After(300 * time.Millisecond)
	for immediate < 3 {
		select {
		case code := <-codes:
			if code != http.StatusServiceUnavailable {
				t.Fatalf("got %d before any slot was released, want 503", code)
			}
			immediate++
		case <-timeout:
			t.Fatalf("only %d requests were rejected immediately, want 3", immediate)
		}
	}
	select {
	case code := <-codes:
		t.Fatalf("a request answered %d before the wait window ended", code)
	case <-time.After(100 * time.Millisecond):
	}
}
