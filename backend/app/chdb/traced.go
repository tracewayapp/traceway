//go:build telemetry_ch

package chdb

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/tracewayapp/traceway/backend/v2/app/db/dbtrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type tracedConn struct {
	inner ChConn
	opts  dbtrace.Options
}

// ClickHouse streams a result block by block, so a read's span stays open
// until the rows are exhausted or closed.
func (c tracedConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	span, ok := dbtrace.StartSpan(ctx, c.opts, query)
	rows, err := c.inner.Query(ctx, query, args...)
	if !ok {
		return rows, err
	}
	if err != nil {
		dbtrace.EndSpan(span, err)
		return rows, err
	}
	return &tracedRows{Rows: rows, span: span}, nil
}

func (c tracedConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	span, ok := dbtrace.StartSpan(ctx, c.opts, query)
	row := c.inner.QueryRow(ctx, query, args...)
	if !ok {
		return row
	}
	if err := row.Err(); err != nil {
		dbtrace.EndSpan(span, err)
		return row
	}
	return &tracedRow{Row: row, span: span}
}

func (c tracedConn) Exec(ctx context.Context, query string, args ...any) error {
	span, ok := dbtrace.StartSpan(ctx, c.opts, query)
	err := c.inner.Exec(ctx, query, args...)
	if ok {
		dbtrace.EndSpan(span, err)
	}
	return err
}

func (c tracedConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	batch, err := c.inner.PrepareBatch(ctx, query, opts...)
	if err != nil {
		return nil, err
	}
	return &tracedBatch{Batch: batch, ctx: ctx, opts: c.opts, query: query}, nil
}

func (c tracedConn) Stats() driver.Stats {
	return c.inner.Stats()
}

type tracedRows struct {
	driver.Rows
	span  trace.Span
	ended bool
}

func (r *tracedRows) Next() bool {
	if r.Rows.Next() {
		return true
	}
	r.end()
	return false
}

func (r *tracedRows) Close() error {
	err := r.Rows.Close()
	r.end()
	return err
}

func (r *tracedRows) end() {
	if !r.ended {
		r.ended = true
		dbtrace.EndSpan(r.span, r.Rows.Err())
	}
}

type tracedRow struct {
	driver.Row
	span  trace.Span
	ended bool
}

func (r *tracedRow) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	r.end(err)
	return err
}

func (r *tracedRow) ScanStruct(dest any) error {
	err := r.Row.ScanStruct(dest)
	r.end(err)
	return err
}

func (r *tracedRow) end(err error) {
	if !r.ended {
		r.ended = true
		dbtrace.EndSpan(r.span, err)
	}
}

type tracedBatch struct {
	driver.Batch
	ctx   context.Context
	opts  dbtrace.Options
	query string
}

func (b *tracedBatch) Send() error {
	span, ok := dbtrace.StartSpan(b.ctx, b.opts, b.query)
	if !ok {
		return b.Batch.Send()
	}
	span.SetAttributes(attribute.Int("db.operation.batch.size", b.Batch.Rows()))
	err := b.Batch.Send()
	dbtrace.EndSpan(span, err)
	return err
}
