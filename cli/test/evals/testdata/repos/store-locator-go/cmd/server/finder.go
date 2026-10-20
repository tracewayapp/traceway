package main

import (
	"context"

	"example.com/store-locator/internal/maps"
	"example.com/store-locator/internal/stores"
)

// postgisFinder is the PostGIS-backed finder in production builds; this
// stand-in keeps the module dependency-free.
type postgisFinder struct{ dsn string }

func newFinder(dsn string) *postgisFinder { return &postgisFinder{dsn: dsn} }

func (f *postgisFinder) Near(ctx context.Context, p maps.Point, limit int) ([]stores.Store, error) {
	return nil, nil
}
