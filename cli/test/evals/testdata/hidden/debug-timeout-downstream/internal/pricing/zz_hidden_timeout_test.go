package pricing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/cart-api/internal/config"
)

// The pricing service answers in 0.25-4.2s (p50 1.1s, p95 2.4s, p99 about
// 3.3s); a quote that takes 2.8s is ordinary and must not fail.
func TestHiddenQuoteSurvivesOrdinaryPricingLatency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2800 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(Quote{TotalCents: 4599, Currency: "USD"})
	}))
	defer srv.Close()
	cfg, err := config.Load(func(k string) string {
		if k == "PRICING_URL" {
			return srv.URL
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	q, err := New(cfg).Quote(context.Background(), "c_1")
	if err != nil {
		t.Fatalf("a 2.8s pricing response failed with the default configuration: %v", err)
	}
	if q.TotalCents != 4599 {
		t.Fatalf("unexpected quote %+v", q)
	}
}

func TestHiddenPricingTimeoutStaysBounded(t *testing.T) {
	cfg, err := config.Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PricingTimeout <= 0 || cfg.PricingTimeout > 30*time.Second {
		t.Fatalf("the pricing call needs a finite timeout (got %s)", cfg.PricingTimeout)
	}
}
