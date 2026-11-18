package db

import (
	"context"
	"database/sql"

	"github.com/tracewayapp/lit/v2"
)

type boundExecutor struct {
	conn *sql.DB
	ctx  context.Context
}

// BindContext lets lit's context-free calls on conn carry ctx's request or
// task span to the tracing driver. Cancellation is unchanged: lit never had
// a context, so the binding drops it.
func BindContext(ctx context.Context, conn *sql.DB) lit.Executor {
	if ctx == nil {
		ctx = context.Background()
	}
	return boundExecutor{conn: conn, ctx: context.WithoutCancel(ctx)}
}

func (e boundExecutor) Exec(query string, args ...any) (sql.Result, error) {
	return e.conn.ExecContext(e.ctx, query, args...)
}

func (e boundExecutor) Query(query string, args ...any) (*sql.Rows, error) {
	return e.conn.QueryContext(e.ctx, query, args...)
}

func (e boundExecutor) QueryRow(query string, args ...any) *sql.Row {
	return e.conn.QueryRowContext(e.ctx, query, args...)
}
