package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type countingStore struct{ listings atomic.Int32 }

func (s *countingStore) CategoryListing(_ context.Context, slug string) ([]Product, error) {
	s.listings.Add(1)
	return []Product{{ID: 1, Slug: slug + "-1", Name: "One", Available: 3}, {ID: 2, Slug: slug + "-2", Name: "Two", Available: 40}}, nil
}

func (s *countingStore) Product(context.Context, int64) (*Product, error) { return nil, nil }

func TestCacheLoadsOnceAndServesFromMemory(t *testing.T) {
	st := &countingStore{}
	h := NewHandlers(st, NewCache(st, 0))
	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/api/categories/shoes/products", nil)
		req.SetPathValue("slug", "shoes")
		rec := httptest.NewRecorder()
		h.Listing(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"badge":"low-stock"`) {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	}
	if n := st.listings.Load(); n != 1 {
		t.Fatalf("listing query ran %d times, want 1", n)
	}
}

func TestPreloadSkipsLoadedCategories(t *testing.T) {
	st := &countingStore{}
	c := NewCache(st, 0)
	if _, err := c.Get(context.Background(), "bags"); err != nil {
		t.Fatal(err)
	}
	c.Preload(context.Background(), []string{"bags", "hats"}, 0)
	if n := st.listings.Load(); n != 2 {
		t.Fatalf("listing query ran %d times, want 2", n)
	}
}
