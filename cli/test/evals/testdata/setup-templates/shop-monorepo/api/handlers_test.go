package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetProduct(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/products/2", nil)
	req.SetPathValue("id", "2")
	rec := httptest.NewRecorder()
	getProduct(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Logo tee") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestAddCartItemValidates(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/cart/items", strings.NewReader(`{"productId":"1","qty":0}`))
	rec := httptest.NewRecorder()
	addCartItem(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
}
