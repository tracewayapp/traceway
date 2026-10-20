package blob

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type hiddenStorage struct {
	creds, store      *httptest.Server
	credHits, putHits atomic.Int32
	credStatus        atomic.Int32
	credExpiresIn     time.Duration
	hang              chan struct{}
}

func newHiddenStorage(t *testing.T, expiresIn time.Duration) *hiddenStorage {
	h := &hiddenStorage{credExpiresIn: expiresIn, hang: make(chan struct{})}
	h.credStatus.Store(http.StatusOK)
	h.creds = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.credHits.Add(1)
		switch status := int(h.credStatus.Load()); status {
		case 0:
			select {
			case <-h.hang:
			case <-r.Context().Done():
			}
		case http.StatusOK:
			_ = json.NewEncoder(w).Encode(map[string]any{"keyId": "k1", "secret": "s1", "expires": time.Now().Add(h.credExpiresIn)})
		default:
			http.Error(w, "unavailable", status)
		}
	}))
	h.store = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.putHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		close(h.hang)
		h.creds.Close()
		h.store.Close()
	})
	return h
}

func (h *hiddenStorage) client() *Client { return New(h.store.URL, "reports-exports", h.creds.URL) }

func put(c *Client) error {
	return c.Put(context.Background(), "exports/7/20260504T083000Z.csv", "text/csv", []byte("a,b\n1,2\n"))
}

func TestHiddenCredentialFailureStillFailsOnce(t *testing.T) {
	h := newHiddenStorage(t, time.Hour)
	h.credStatus.Store(http.StatusServiceUnavailable)
	err := put(h.client())
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("a 503 from the credential endpoint must fail the upload and name the status, got %v", err)
	}
	if n := h.credHits.Load(); n != 1 {
		t.Fatalf("the credential endpoint was called %d times for one upload; the storage client's behaviour must not change", n)
	}
	if n := h.putHits.Load(); n != 0 {
		t.Fatalf("the upload went ahead without credentials (%d PUTs)", n)
	}
}

func TestHiddenCredentialsCachedUntilAMinuteBeforeExpiry(t *testing.T) {
	h := newHiddenStorage(t, 90*time.Second)
	c := h.client()
	for i := 0; i < 2; i++ {
		if err := put(c); err != nil {
			t.Fatal(err)
		}
	}
	if n := h.credHits.Load(); n != 1 {
		t.Fatalf("credentials valid for another 90s were fetched %d times for two uploads", n)
	}
}

func TestHiddenExpiringCredentialsStillRefreshed(t *testing.T) {
	h := newHiddenStorage(t, 30*time.Second)
	c := h.client()
	if err := put(c); err != nil {
		t.Fatal(err)
	}
	h.credStatus.Store(http.StatusServiceUnavailable)
	if err := put(c); err == nil {
		t.Fatal("credentials that expire within a minute must be refreshed, and a failed refresh must fail the upload")
	}
	if n := h.credHits.Load(); n != 2 {
		t.Fatalf("the credential endpoint was called %d times, want 2 (one fetch per upload)", n)
	}
	if n := h.putHits.Load(); n != 1 {
		t.Fatalf("%d PUTs, want only the one that had fresh credentials", n)
	}
}

func TestHiddenCredentialTimeoutUnchanged(t *testing.T) {
	h := newHiddenStorage(t, time.Hour)
	h.credStatus.Store(0)
	c := h.client()
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- put(c) }()
	select {
	case err := <-done:
		if took := time.Since(start); err == nil || took < 1500*time.Millisecond {
			t.Fatalf("a hanging credential endpoint must fail the upload after the 2s timeout, got %v after %s", err, took.Round(time.Millisecond))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a hanging credential endpoint held the upload for more than 5s; the credential fetch needs its 2s timeout")
	}
}
