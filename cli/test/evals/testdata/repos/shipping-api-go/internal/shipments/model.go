// Package shipments books shipments for paid orders.
package shipments

// Address is a destination address.
type Address struct {
	Name       string `json:"name"`
	Line1      string `json:"line1"`
	City       string `json:"city"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"` // ISO 3166-1 alpha-2
}

// CreateRequest is the body of POST /api/shipments.
type CreateRequest struct {
	OrderID     string  `json:"orderId"`
	Carrier     string  `json:"carrier"`
	WeightGrams int     `json:"weightGrams"`
	To          Address `json:"to"`
}

// Shipment is a booked shipment.
type Shipment struct {
	ID         string `json:"id"`
	OrderID    string `json:"orderId"`
	Carrier    string `json:"carrier"`
	PriceCents int64  `json:"priceCents"`
}
