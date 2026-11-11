// Package dbtrace traces statements under a request or task span. lit issues
// every query without a context, so a statement inside a transaction takes
// its parent from the context the transaction was begun with.
package dbtrace

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"time"

	"github.com/tracewayapp/traceway/backend/app/traceway"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Options struct {
	System    string
	Namespace string
}

func Open(driverName, dsn string, opts Options) (*sql.DB, error) {
	plain, err := sql.Open(driverName, dsn)
	if err != nil || !traceway.Enabled() {
		return plain, err
	}
	d := plain.Driver()
	plain.Close()

	var connector driver.Connector = dsnConnector{dsn: dsn, driver: d}
	if dc, ok := d.(driver.DriverContext); ok {
		if connector, err = dc.OpenConnector(dsn); err != nil {
			return nil, err
		}
	}
	return sql.OpenDB(WrapConnector(connector, opts)), nil
}

func WrapConnector(c driver.Connector, opts Options) driver.Connector {
	if !traceway.Enabled() {
		return c
	}
	return &tracedConnector{inner: c, opts: opts}
}

type dsnConnector struct {
	dsn    string
	driver driver.Driver
}

func (c dsnConnector) Connect(context.Context) (driver.Conn, error) {
	return c.driver.Open(c.dsn)
}

func (c dsnConnector) Driver() driver.Driver {
	return c.driver
}

type tracedConnector struct {
	inner driver.Connector
	opts  Options
}

func (c *tracedConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &tracedConn{inner: conn, opts: c.opts}, nil
}

func (c *tracedConnector) Driver() driver.Driver {
	return c.inner.Driver()
}

func (c *tracedConnector) Close() error {
	if closer, ok := c.inner.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// database/sql never uses one driver.Conn from two goroutines at once, so the
// transaction state needs no lock.
type tracedConn struct {
	inner driver.Conn
	opts  Options
	tx    *txState
}

type txState struct {
	parent context.Context
	batch  *batchSpan
}

// One INSERT per row would otherwise be thousands of spans per ingest
// request. The folded span lasts as long as its statements spent in the
// driver together, so the Go work between rows is not counted as database
// time.
type batchSpan struct {
	span  trace.Span
	query string
	size  int
	start time.Time
	busy  time.Duration
}

func (c *tracedConn) Prepare(query string) (driver.Stmt, error) {
	return c.inner.Prepare(query)
}

func (c *tracedConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	var stmt driver.Stmt
	var err error
	if p, ok := c.inner.(driver.ConnPrepareContext); ok {
		stmt, err = p.PrepareContext(ctx, query)
	} else {
		stmt, err = c.inner.Prepare(query)
	}
	if err != nil {
		return nil, err
	}
	return newTracedStmt(stmt, c, query), nil
}

func (c *tracedConn) Close() error {
	c.endTx()
	return c.inner.Close()
}

func (c *tracedConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *tracedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	var tx driver.Tx
	var err error
	if b, ok := c.inner.(driver.ConnBeginTx); ok {
		tx, err = b.BeginTx(ctx, opts)
	} else {
		tx, err = c.inner.Begin() //nolint:staticcheck // fallback for drivers without BeginTx
	}
	if err != nil {
		return nil, err
	}
	parent, _ := traceway.StatementParent(ctx)
	c.tx = &txState{parent: parent}
	return &tracedTx{inner: tx, conn: c}, nil
}

func (c *tracedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.inner.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return c.exec(ctx, query, func() (driver.Result, error) {
		return execer.ExecContext(ctx, query, args)
	})
}

func (c *tracedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.inner.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return c.query(ctx, query, func() (driver.Rows, error) {
		return queryer.QueryContext(ctx, query, args)
	})
}

func (c *tracedConn) Ping(ctx context.Context) error {
	if p, ok := c.inner.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func (c *tracedConn) ResetSession(ctx context.Context) error {
	c.endTx()
	if r, ok := c.inner.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}

func (c *tracedConn) IsValid() bool {
	if v, ok := c.inner.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

func (c *tracedConn) CheckNamedValue(nv *driver.NamedValue) error {
	if checker, ok := c.inner.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

func (c *tracedConn) parent(ctx context.Context) (context.Context, bool) {
	if parent, ok := traceway.StatementParent(ctx); ok {
		return parent, true
	}
	if c.tx != nil && c.tx.parent != nil {
		return c.tx.parent, true
	}
	return nil, false
}

func (c *tracedConn) exec(ctx context.Context, query string, run func() (driver.Result, error)) (driver.Result, error) {
	parent, ok := c.parent(ctx)
	if !ok {
		return run()
	}
	if c.tx == nil {
		span := startSpan(parent, c.opts, query)
		result, err := run()
		EndSpan(span, err)
		return result, err
	}

	begin := time.Now()
	if b := c.tx.batch; b != nil && b.query == query {
		result, err := run()
		b.size++
		b.busy += time.Since(begin)
		recordError(b.span, err)
		return result, err
	}
	c.endBatch()
	span := startSpan(parent, c.opts, query, trace.WithTimestamp(begin))
	result, err := run()
	c.tx.batch = &batchSpan{span: span, query: query, size: 1, start: begin, busy: time.Since(begin)}
	recordError(span, err)
	return result, err
}

// A read's span stays open until its rows are closed: the drivers return
// after the first row, and the rest of the result is read during Next.
func (c *tracedConn) query(ctx context.Context, query string, run func() (driver.Rows, error)) (driver.Rows, error) {
	parent, ok := c.parent(ctx)
	if !ok {
		return run()
	}
	c.endBatch()
	span := startSpan(parent, c.opts, query)
	rows, err := run()
	if err != nil {
		EndSpan(span, err)
		return nil, err
	}
	return newTracedRows(rows, span), nil
}

func (c *tracedConn) endBatch() {
	if c.tx == nil || c.tx.batch == nil {
		return
	}
	b := c.tx.batch
	if b.size > 1 {
		b.span.SetAttributes(attribute.Int("db.operation.batch.size", b.size))
	}
	b.span.End(trace.WithTimestamp(b.start.Add(b.busy)))
	c.tx.batch = nil
}

func (c *tracedConn) endTx() {
	c.endBatch()
	c.tx = nil
}

type tracedTx struct {
	inner driver.Tx
	conn  *tracedConn
}

func (t *tracedTx) Commit() error {
	t.conn.endTx()
	return t.inner.Commit()
}

func (t *tracedTx) Rollback() error {
	t.conn.endTx()
	return t.inner.Rollback()
}
