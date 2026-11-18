package invoices

import "fmt"

// View is the invoice as the page shows it.
type View struct {
	Number         string     `json:"number"`
	IssuedAt       string     `json:"issuedAt"`
	CustomerName   string     `json:"customerName"`
	BillingAddress string     `json:"billingAddress"`
	Lines          []ViewLine `json:"lines"`
	Total          string     `json:"total"`
}

// ViewLine is one rendered line.
type ViewLine struct {
	Description string `json:"description"`
	Qty         int    `json:"qty"`
	Amount      string `json:"amount"`
}

func render(inv *Invoice) View {
	v := View{
		Number:         inv.Number,
		IssuedAt:       inv.IssuedAt.Format("2 January 2006"),
		CustomerName:   inv.Customer.Name,
		BillingAddress: formatAddress(inv.Customer.BillingAddress),
		Total:          money(inv.TotalCents, inv.Currency),
	}
	for _, l := range inv.Lines {
		v.Lines = append(v.Lines, ViewLine{Description: l.Description, Qty: l.Qty, Amount: money(int64(l.Qty)*l.UnitCents, inv.Currency)})
	}
	return v
}

func formatAddress(a *Address) string {
	s := a.Line1
	if a.Line2 != "" {
		s += ", " + a.Line2
	}
	return fmt.Sprintf("%s, %s %s, %s", s, a.PostalCode, a.City, a.Country)
}

func money(cents int64, currency string) string {
	return fmt.Sprintf("%d.%02d %s", cents/100, cents%100, currency)
}
