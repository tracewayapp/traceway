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

// Handle buys one shipping label per package of a paid order.
func (s *Service) Handle(ctx context.Context, ev OrderPaid) error {
	order, err := s.orders.Get(ctx, ev.OrderID)
	if err != nil {
		return fmt.Errorf("load order %s: %w", ev.OrderID, err)
	}
	for i, p := range order.Packages {
		req := LabelRequest{OrderID: order.ID, To: order.ShipTo, Service: p.Service, Parcels: []Parcel{{WeightGrams: p.WeightGrams}}}
		label, err := s.carrier.CreateLabel(ctx, req)
		if err != nil {
			return fmt.Errorf("create label %d for order %s: %w", i+1, order.ID, err)
		}
		if err := s.store.SaveLabel(ctx, order.ID, label); err != nil {
			return err
		}
	}
	return nil
}
