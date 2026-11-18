package invoices

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type hiddenStore struct{ inv *Invoice }

func (h hiddenStore) Invoice(context.Context, int64) (*Invoice, error) { return h.inv, nil }

func TestHiddenRenderWithoutBillingAddress(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("render panics for a customer without a billing address: %v", r)
		}
	}()
	v := render(&Invoice{Number: "INV-7", TotalCents: 990, Currency: "EUR", Customer: &Customer{Name: "Bo"}})
	if v.CustomerName != "Bo" || v.Total != "9.90 EUR" {
		t.Fatalf("unexpected view %+v", v)
	}
	if strings.Contains(v.BillingAddress, "<nil>") || strings.HasPrefix(strings.TrimSpace(v.BillingAddress), ",") {
		t.Fatalf("a missing address must not render as %q", v.BillingAddress)
	}
}

func TestHiddenGetWithoutBillingAddress(t *testing.T) {
	h := NewHandler(hiddenStore{inv: &Invoice{Number: "INV-8", Currency: "EUR", Customer: &Customer{Name: "Cy"}}})
	req := httptest.NewRequest(http.MethodGet, "/api/invoices/8", nil)
	req.SetPathValue("id", "8")
	rec := httptest.NewRecorder()
	Recover(http.HandlerFunc(h.Get)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/invoices/8 for a customer without a billing address answered %d", rec.Code)
	}
}

func TestHiddenRenderWithAddressUnchanged(t *testing.T) {
	v := render(&Invoice{Currency: "EUR", Customer: &Customer{Name: "Ada", BillingAddress: &Address{Line1: "Main St 1", City: "Berlin", PostalCode: "10115", Country: "DE"}}})
	if v.BillingAddress != "Main St 1, 10115 Berlin, DE" {
		t.Fatalf("the address format changed: %q", v.BillingAddress)
	}
}
