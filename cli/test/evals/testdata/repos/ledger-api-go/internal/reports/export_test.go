package reports

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeSource []Entry

func (f fakeSource) Entries(_ context.Context, _ int64, from, to time.Time, each func(Entry) error) error {
	for _, e := range f {
		if !e.PostedAt.Before(from) && e.PostedAt.Before(to) {
			if err := each(e); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestExportWritesCSV(t *testing.T) {
	src := fakeSource{
		{ID: 1, PostedAt: time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC), AccountCode: "4000", Counterparty: "Acme", Description: "Invoice 1001", AmountCents: 125050, Currency: "EUR"},
		{ID: 2, PostedAt: time.Date(2026, 7, 3, 9, 0, 0, 0, time.UTC), AccountCode: "1200", Counterparty: "Acme", Description: "Refund", AmountCents: -999, Currency: "EUR"},
		{ID: 3, PostedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), AccountCode: "4000", Counterparty: "Other", Description: "Next quarter", AmountCents: 100, Currency: "EUR"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/reports/export?from=2026-07-01&to=2026-10-01", nil)
	req.Header.Set("X-Org-Id", "7")
	rec := httptest.NewRecorder()
	NewExporter(src).Export(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want header + 2:\n%s", len(lines), rec.Body.String())
	}
	if !strings.Contains(lines[1], "1250.50") || !strings.Contains(lines[2], "-9.99") {
		t.Errorf("amounts not formatted: %q", lines[1:])
	}
}

func TestExportRejectsBadRange(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/reports/export?from=2026-10-01&to=2026-07-01", nil)
	req.Header.Set("X-Org-Id", "7")
	rec := httptest.NewRecorder()
	NewExporter(fakeSource{}).Export(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}
