package main

import (
	"context"
	"os"

	"example.com/fulfillment/internal/carrier"
	"example.com/fulfillment/internal/store"
	"example.com/fulfillment/internal/telemetry"
)

type deps struct {
	orders  *store.Orders
	carrier *carrier.Client
	store   *store.Labels
}

func connect(ctx context.Context) (*deps, error) {
	db, err := telemetry.OpenDB("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return &deps{
		orders:  store.NewOrders(db),
		carrier: carrier.New(os.Getenv("CARRIER_API_URL"), os.Getenv("CARRIER_API_KEY")),
		store:   store.NewLabels(db),
	}, nil
}
