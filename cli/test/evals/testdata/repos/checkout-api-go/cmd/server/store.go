package main

import (
	"context"

	"example.com/checkout-api/internal/checkout"
)

// pgStore is the pgx-backed store in production builds; this stand-in keeps
// the module dependency-free.
type pgStore struct{ dsn string }

func newStore(dsn string) *pgStore { return &pgStore{dsn: dsn} }

func (s *pgStore) Order(ctx context.Context, id, customerID int64) (*checkout.Order, error) {
	return nil, nil
}

func (s *pgStore) OrderByID(ctx context.Context, id int64) (*checkout.Order, error) {
	return nil, nil
}

func (s *pgStore) MarkPaid(ctx context.Context, id int64, chargeID string) error { return nil }

func (s *pgStore) RecordRefund(ctx context.Context, id int64) error { return nil }
