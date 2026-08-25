package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func admissionRouter(gate gin.HandlerFunc, release chan struct{}) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", func(c *gin.Context) {
		if h := c.GetHeader("X-Project"); h != "" {
			c.Set(ProjectIdContextKey, uuid.NewSHA1(uuid.NameSpaceOID, []byte(h)))
		}
	}, gate, func(c *gin.Context) {
		if release != nil {
			<-release
		}
		c.Status(http.StatusOK)
	})
	return r
}

func postAs(r *gin.Engine, project string) int {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	if project != "" {
		req.Header.Set("X-Project", project)
	}
	r.ServeHTTP(rec, req)
	return rec.Code
}

func admitted(r *gin.Engine, project string) bool {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Project", project)
	done := make(chan struct{})
	go func() {
		r.ServeHTTP(rec, req)
		close(done)
	}()
	select {
	case <-done:
		return rec.Code == http.StatusOK
	case <-time.After(200 * time.Millisecond):
		return true
	}
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
			codes[i] = postAs(r, "")
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

func TestAdmissionGateReservesCapacityAcrossProjects(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	r := admissionRouter(newAdmissionGate(4, 50*time.Millisecond, "busy", nil), release)

	hog := make(chan int, 8)
	for i := 0; i < 8; i++ {
		go func() { hog <- postAs(r, "hog") }()
	}
	time.Sleep(150 * time.Millisecond)

	if !admitted(r, "victim") {
		t.Fatal("victim was rejected while the hog was saturating the gate")
	}

	for i := 0; i < 5; i++ {
		select {
		case code := <-hog:
			if code != http.StatusServiceUnavailable {
				t.Fatalf("hog request got %d, want 503", code)
			}
		case <-time.After(time.Second):
			t.Fatalf("only %d hog requests were rejected, want 5", i)
		}
	}
}

func TestAdmissionGateBoundsWaiters(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	gate := &admissionGate{
		slots:   make(chan struct{}, 1),
		waiters: make(chan struct{}, 2),
		wait:    time.Second,
		message: "busy",
	}
	r := admissionRouter(gate.handle, release)

	codes := make(chan int, 8)
	for i := 0; i < 6; i++ {
		go func() { codes <- postAs(r, "") }()
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
		t.Fatalf("a fourth request answered %d early", code)
	case <-time.After(100 * time.Millisecond):
	}
}
