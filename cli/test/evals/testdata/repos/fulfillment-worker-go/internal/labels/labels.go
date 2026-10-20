package labels

import (
	"context"
	"fmt"
)

// Orders loads orders.
type Orders interface {
	Get(ctx context.Context, id string) (*Order, error)
}

// Carrier buys labels.
type Carrier interface {
	CreateLabel(ctx context.Context, req LabelRequest) (*Label, error)
}

// Store records bought labels.
type Store interface {
	SaveLabel(ctx context.Context, orderID string, l *Label) error
}

// Service handles paid orders.
type Service struct {
	orders  Orders
	carrier Carrier
	store   Store
}

// NewService returns a Service.
func NewService(o Orders, c Carrier, s Store) *Service {
	return &Service{orders: o, carrier: c, store: s}
}

// Handle buys the shipping label for a paid order.
func (s *Service) Handle(ctx context.Context, ev OrderPaid) error {
	order, err := s.orders.Get(ctx, ev.OrderID)
	if err != nil {
		return fmt.Errorf("load order %s: %w", ev.OrderID, err)
	}
	req := LabelRequest{OrderID: order.ID, To: order.ShipTo, Service: order.Packages[0].Service}
	for _, p := range order.Packages {
		req.Parcels = append(req.Parcels, Parcel{WeightGrams: p.WeightGrams})
	}
	label, err := s.carrier.CreateLabel(ctx, req)
	if err != nil {
		return fmt.Errorf("create label for order %s: %w", order.ID, err)
	}
	return s.store.SaveLabel(ctx, order.ID, label)
}
