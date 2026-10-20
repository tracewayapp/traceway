package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxPool adapts a *pgxpool.Pool to Pool.
type PgxPool struct{ *pgxpool.Pool }

// Begin starts a transaction on a pooled connection.
func (p PgxPool) Begin(ctx context.Context) (Tx, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return nil, convert(err)
	}
	return pgxTx{tx}, nil
}

type pgxTx struct{ tx pgx.Tx }

func (t pgxTx) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := t.tx.Exec(ctx, sql, args...)
	if err != nil {
		return 0, convert(err)
	}
	return tag.RowsAffected(), nil
}

func (t pgxTx) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return pgxRow{t.tx.QueryRow(ctx, sql, args...)}
}

func (t pgxTx) Commit(ctx context.Context) error   { return convert(t.tx.Commit(ctx)) }
func (t pgxTx) Rollback(ctx context.Context) error { return convert(t.tx.Rollback(ctx)) }

type pgxRow struct{ row pgx.Row }

func (r pgxRow) Scan(dest ...any) error { return convert(r.row.Scan(dest...)) }

// convert turns pgx's errors into this package's and returns any other
// error unchanged.
func convert(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoRows
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return &PgError{Code: pg.Code, Message: pg.Message, Detail: pg.Detail, Where: pg.Where}
	}
	return err
}
