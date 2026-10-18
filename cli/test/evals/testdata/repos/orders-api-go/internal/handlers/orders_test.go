package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/orders-api/internal/store"
)

type fakeStore struct {
	orders map[int64][]store.Order
	items  map[int64][]store.Item
}

func (f *fakeStore) OrdersByUser(_ context.Context, userID int64) ([]store.Order, error) {
	return append([]store.Order{}, f.orders[userID]...), nil
}

func (f *fakeStore) OrderByID(_ context.Context, id int64) (*store.Order, error) {
	for _, os := range f.orders {
		for _, o := range os {
			if o.ID == id {
				return &o, nil
			}
		}
	}
	return nil, nil
}

func (f *fakeStore) ItemsByOrder(_ context.Context, orderID int64) ([]store.Item, error) {
	return f.items[orderID], nil
}

func TestListOrdersIncludesItems(t *testing.T) {
	fs := &fakeStore{
		orders: map[int64][]store.Order{7: {{ID: 1, UserID: 7}, {ID: 2, UserID: 7}}},
		items:  map[int64][]store.Item{1: {{ID: 10, SKU: "A"}}, 2: {{ID: 11, SKU: "B"}, {ID: 12, SKU: "C"}}},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	req.Header.Set("X-User-Id", "7")
	rec := httptest.NewRecorder()
	New(fs).ListOrders(rec, req)
	var got []store.Order
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || len(got[0].Items) != 1 || len(got[1].Items) != 2 {
		t.Fatalf("unexpected orders: %+v", got)
	}
}

func TestListOrdersNeedsUser(t *testing.T) {
	rec := httptest.NewRecorder()
	New(&fakeStore{}).ListOrders(rec, httptest.NewRequest(http.MethodGet, "/api/orders", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}
