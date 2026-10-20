// Package db is the seam between the handlers and pgx: the handlers use these
// interfaces, and pgx.go adapts *pgxpool.Pool and pgx.Tx to them.
package db

import (
	"context"
	"errors"
)

// Tx is the part of pgx.Tx the handlers use. Exec returns the rows affected.
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// Row is one result row.
type Row interface {
	Scan(dest ...any) error
}

// Pool begins transactions.
type Pool interface {
	Begin(ctx context.Context) (Tx, error)
}

// Config is the pgxpool configuration.
var Config = struct {
	MaxConns          int32
	MinConns          int32
	MaxConnIdleTimeMs int
}{
	MaxConns:          40,
	MinConns:          4,
	MaxConnIdleTimeMs: 300_000,
}

// ErrNoRows is returned by Row.Scan when the query matched no row.
var ErrNoRows = errors.New("no rows in result set")

// PgError is the subset of *pgconn.PgError the handlers inspect.
type PgError struct {
	Code    string
	Message string
	Detail  string
	Where   string
}

func (e *PgError) Error() string { return "ERROR: " + e.Message + " (SQLSTATE " + e.Code + ")" }

// SQLState returns the SQLSTATE of a Postgres error, or "".
func SQLState(err error) string {
	var pg *PgError
	if errors.As(err, &pg) {
		return pg.Code
	}
	return ""
}
