package exports

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	invoices []Invoice
}

func (f fakeStore) InvoicesWithLines(context.Context, int64, time.Time, time.Time) ([]Invoice, error) {
	out := make([]Invoice, len(f.invoices))
	copy(out, f.invoices)
	return out, nil
}

func TestInvoicesExport(t *testing.T) {
	st := fakeStore{
		invoices: []Invoice{
			{ID: 1, Number: "INV-1001", IssuedAt: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), CustomerName: "Acme", TotalCents: 11900,
				Lines: []Line{{SKU: "A-1", Description: "Seat", Qty: 1, UnitPriceCents: 10000}}},
			{ID: 2, Number: "INV-1002", IssuedAt: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC), CustomerName: "Globex", TotalCents: 2380,
				Lines: []Line{{SKU: "B-1", Description: "Add-on", Qty: 2, UnitPriceCents: 1000}}},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/exports/invoices?month=2026-09", nil)
	req.Header.Set("X-Org-Id", "3")
	rec := httptest.NewRecorder()
	New(st).Invoices(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"INV-1001,2026-09-02,Acme,A-1,Seat,1,100.00,119.00", "INV-1002,2026-09-03,Globex,B-1,Add-on,2,10.00,23.80"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing row %q in:\n%s", want, body)
		}
	}
}
