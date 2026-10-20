//go:build !pgx

package main

import (
	"errors"

	"example.com/inventory-api/internal/db"
)

// openPool is provided by the pgx build (go run -tags pgx ./cmd/server),
// which wires pgxpool with the otelpgx tracer.
func openPool(string, int32) (db.Pool, error) {
	return nil, errors.New("built without -tags pgx")
}
