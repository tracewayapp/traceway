// Package exports serves the accounting exports.
package exports

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/invoicing-api/internal/telemetry"
)

// Handlers serves /api/exports.
type Handlers struct{ store Store }

// New returns the export handlers.
func New(s Store) *Handlers { return &Handlers{store: s} }

// Invoices serves GET /api/exports/invoices?month=YYYY-MM: one CSV row per
// invoice line, with the invoice's tax breakdown in the last column.
func (h *Handlers) Invoices(w http.ResponseWriter, r *http.Request) {
	orgID, err := strconv.ParseInt(r.Header.Get("X-Org-Id"), 10, 64)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	from, err := time.Parse("2006-01", r.URL.Query().Get("month"))
	if err != nil {
		http.Error(w, "month must look like 2026-09", http.StatusBadRequest)
		return
	}
	to := from.AddDate(0, 1, 0)
	ctx := r.Context()
	invs, err := h.store.InvoicesWithLines(ctx, orgID, from, to)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	for i := range invs {
		taxes, err := h.store.TaxLines(ctx, invs[i].ID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		invs[i].Taxes = taxes
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="invoices-`+from.Format("2006-01")+`.csv"`)
	_, end := telemetry.Start(ctx, "encode csv")
	defer end()
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"invoice", "issued_at", "customer", "sku", "description", "qty", "unit_price", "invoice_total", "taxes"})
	for _, inv := range invs {
		taxes := formatTaxes(inv.Taxes)
		for _, l := range inv.Lines {
			_ = cw.Write([]string{
				inv.Number, inv.IssuedAt.UTC().Format(time.DateOnly), inv.CustomerName, l.SKU, l.Description,
				strconv.FormatInt(l.Qty, 10), cents(l.UnitPriceCents), cents(inv.TotalCents), taxes,
			})
		}
	}
	cw.Flush()
}

func formatTaxes(ts []TaxLine) string {
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		parts = append(parts, t.RateCode+"="+cents(t.AmountCents))
	}
	return strings.Join(parts, ";")
}

func cents(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	frac := strconv.FormatInt(c%100, 10)
	if len(frac) == 1 {
		frac = "0" + frac
	}
	return sign + strconv.FormatInt(c/100, 10) + "." + frac
}
