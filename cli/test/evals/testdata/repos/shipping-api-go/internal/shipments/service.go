package shipments

import (
	"context"
	"fmt"
)

// Rater prices a parcel.
type Rater interface {
	Quote(ctx context.Context, carrier string, zone, weightGrams int) (int64, error)
}

// Repo stores shipments.
type Repo interface {
	Insert(ctx context.Context, s *Shipment) error
	Get(ctx context.Context, id string) (*Shipment, error)
}

// Service books shipments.
type Service struct {
	rates Rater
	repo  Repo
}

// NewService returns a Service.
func NewService(r Rater, repo Repo) *Service { return &Service{rates: r, repo: repo} }

// Create validates, rates and books a shipment.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Shipment, error) {
	if err := validateAddress(req.To); err != nil {
		return nil, fmt.Errorf("validate address: %w", err)
	}
	price, err := s.rates.Quote(ctx, req.Carrier, zoneFor(req.To.Country), req.WeightGrams)
	if err != nil {
		return nil, fmt.Errorf("rate shipment: %w", err)
	}
	sh := &Shipment{OrderID: req.OrderID, Carrier: req.Carrier, PriceCents: price}
	if err := s.repo.Insert(ctx, sh); err != nil {
		return nil, fmt.Errorf("save shipment: %w", err)
	}
	return sh, nil
}

// Get returns a booked shipment, or nil when there is none.
func (s *Service) Get(ctx context.Context, id string) (*Shipment, error) {
	sh, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load shipment: %w", err)
	}
	return sh, nil
}

// zoneFor maps a destination country to the carrier zone.
func zoneFor(country string) int {
	switch country {
	case "US":
		return 1
	case "CA", "MX":
		return 3
	case "GB", "IE", "FR", "DE", "NL", "BE", "ES", "IT", "PT", "AT":
		return 5
	case "AU", "NZ", "JP":
		return 7
	default:
		return 9
	}
}
