// Package invoices serves invoice pages.
package invoices

import "time"

// Invoice is a billed order.
type Invoice struct {
	ID         int64
	Number     string
	IssuedAt   time.Time
	Customer   *Customer
	Lines      []Line
	TotalCents int64
	Currency   string
}

// Customer is who the invoice is billed to.
type Customer struct {
	ID    int64
	Name  string
	Email string
	// BillingAddress is optional: customers who only buy digital goods
	// never enter one.
	BillingAddress *Address
}

// Address is a postal address.
type Address struct {
	Line1      string
	Line2      string
	City       string
	PostalCode string
	Country    string
}

// Line is one invoice line.
type Line struct {
	Description string
	Qty         int
	UnitCents   int64
}
