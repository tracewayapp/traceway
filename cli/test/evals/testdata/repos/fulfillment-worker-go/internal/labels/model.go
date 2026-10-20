// Package labels buys shipping labels for paid orders.
package labels

// OrderPaid is the event published on fulfillment.orders.
type OrderPaid struct {
	OrderID string `json:"orderId"`
}

// Order is what the labels service needs to know about an order.
type Order struct {
	ID       string
	ShipTo   Address
	Packages []Package
}

// Address is a postal address.
type Address struct {
	Name       string
	Line1      string
	City       string
	PostalCode string
	Country    string
}

// Package is one physical parcel of an order. Gift cards and other digital
// items ship nothing, so an order of only those has no packages.
type Package struct {
	WeightGrams int
	Service     string // carrier service level: standard | express
}

// Parcel is one parcel of a label request.
type Parcel struct {
	WeightGrams int `json:"weightGrams"`
}

// LabelRequest asks the carrier for one label covering every parcel.
type LabelRequest struct {
	OrderID string   `json:"orderId"`
	To      Address  `json:"to"`
	Service string   `json:"service"`
	Parcels []Parcel `json:"parcels"`
}

// Label is a bought label.
type Label struct {
	TrackingNumber string `json:"trackingNumber"`
	URL            string `json:"labelUrl"`
}
