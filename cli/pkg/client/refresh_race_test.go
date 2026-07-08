package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// TestDo_concurrentRefreshSingleFlight exercises the MCP server's usage
// pattern: many goroutines hit a 401 on the same expired token at once. The
// refresher must run exactly once (refresh tokens are single-use) and every
// request must succeed with the rotated token. Run with -race to also assert
// the JWT accesses are synchronized.
func TestDo_concurrentRefreshSingleFlight(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fresh" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer srv.Close()

	var refreshCalls atomic.Int32
	c := New(srv.URL, WithJWT("stale"), WithRefresher(func(context.Context) (string, error) {
		refreshCalls.Add(1)
		return "fresh", nil
	}))

	const workers = 16
	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := range workers {
		wg.Go(func() {
			var out struct {
				OK bool `json:"ok"`
			}
			errs[i] = c.do(context.Background(), http.MethodGet, "/api/test", nil, &out)
		})
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("worker %d: %v", i, err)
		}
	}
	if got := refreshCalls.Load(); got != 1 {
		t.Errorf("refresher ran %d times, want exactly 1 (refresh tokens are single-use)", got)
	}
}
