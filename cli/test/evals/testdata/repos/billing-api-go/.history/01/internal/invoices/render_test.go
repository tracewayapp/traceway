package invoices

import (
	"testing"
	"time"
)

func TestRenderWithAddress(t *testing.T) {
	inv := &Invoice{Number: "INV-1042", IssuedAt: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC), TotalCents: 4990, Currency: "EUR",
		Customer: &Customer{Name: "Ada", BillingAddress: &Address{Line1: "Main St 1", City: "Berlin", PostalCode: "10115", Country: "DE"}},
		Lines:    []Line{{Description: "Plan", Qty: 1, UnitCents: 4990}}}
	v := render(inv)
	if v.Total != "49.90 EUR" {
		t.Fatalf("unexpected view %+v", v)
	}
}
