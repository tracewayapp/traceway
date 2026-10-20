package quote

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/cart-api/internal/pricing"
)

type fakePricer struct {
	q   *pricing.Quote
	err error
}

func (f fakePricer) Quote(context.Context, string) (*pricing.Quote, error) { return f.q, f.err }

func TestQuote(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(fakePricer{q: &pricing.Quote{TotalCents: 4599, Currency: "USD"}}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cart/quote?cart=c_1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	NewHandler(fakePricer{err: errors.New("boom")}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cart/quote?cart=c_1", nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("a pricing failure must answer 502, got %d", rec.Code)
	}
}
