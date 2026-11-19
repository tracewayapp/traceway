package dbtrace

import (
	"context"
	"database/sql/driver"
	"errors"
)

type tracedStmt struct {
	inner driver.Stmt
	conn  *tracedConn
	query string
}

func newTracedStmt(stmt driver.Stmt, conn *tracedConn, query string) *tracedStmt {
	return &tracedStmt{inner: stmt, conn: conn, query: query}
}

func (s *tracedStmt) Close() error {
	return s.inner.Close()
}

func (s *tracedStmt) NumInput() int {
	return s.inner.NumInput()
}

func (s *tracedStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.inner.Exec(args) //nolint:staticcheck // required by driver.Stmt
}

func (s *tracedStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.inner.Query(args) //nolint:staticcheck // required by driver.Stmt
}

func (s *tracedStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.conn.exec(ctx, s.query, func() (driver.Result, error) {
		if execer, ok := s.inner.(driver.StmtExecContext); ok {
			return execer.ExecContext(ctx, args)
		}
		values, err := namedToValues(args)
		if err != nil {
			return nil, err
		}
		return s.inner.Exec(values) //nolint:staticcheck // fallback for drivers without StmtExecContext
	})
}

func (s *tracedStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return s.conn.query(ctx, s.query, func() (driver.Rows, error) {
		if queryer, ok := s.inner.(driver.StmtQueryContext); ok {
			return queryer.QueryContext(ctx, args)
		}
		values, err := namedToValues(args)
		if err != nil {
			return nil, err
		}
		return s.inner.Query(values) //nolint:staticcheck // fallback for drivers without StmtQueryContext
	})
}

// database/sql asks the statement first and the connection second; a
// statement wrapper that answered ErrSkip itself would hide the
// connection's checker (DuckDB's custom types live there).
func (s *tracedStmt) CheckNamedValue(nv *driver.NamedValue) error {
	if checker, ok := s.inner.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(nv)
	}
	return s.conn.CheckNamedValue(nv)
}

func namedToValues(named []driver.NamedValue) ([]driver.Value, error) {
	values := make([]driver.Value, len(named))
	for i, nv := range named {
		if nv.Name != "" {
			return nil, errors.New("sql: driver does not support the use of Named Parameters")
		}
		values[i] = nv.Value
	}
	return values, nil
}
