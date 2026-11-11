package dbtrace

import (
	"database/sql/driver"
	"io"
	"reflect"

	"go.opentelemetry.io/otel/trace"
)

// The optional column-type and result-set methods fall back to exactly what
// database/sql assumes when a driver does not implement them, so wrapping
// never changes what ColumnTypes or NextResultSet report.
type tracedRows struct {
	inner   driver.Rows
	span    trace.Span
	readErr error
	ended   bool
}

func newTracedRows(rows driver.Rows, span trace.Span) *tracedRows {
	return &tracedRows{inner: rows, span: span}
}

func (r *tracedRows) Columns() []string {
	return r.inner.Columns()
}

func (r *tracedRows) Next(dest []driver.Value) error {
	err := r.inner.Next(dest)
	if err != nil && err != io.EOF && r.readErr == nil {
		r.readErr = err
	}
	return err
}

func (r *tracedRows) Close() error {
	err := r.inner.Close()
	if !r.ended {
		r.ended = true
		if r.readErr != nil {
			EndSpan(r.span, r.readErr)
		} else {
			EndSpan(r.span, err)
		}
	}
	return err
}

func (r *tracedRows) HasNextResultSet() bool {
	if next, ok := r.inner.(driver.RowsNextResultSet); ok {
		return next.HasNextResultSet()
	}
	return false
}

func (r *tracedRows) NextResultSet() error {
	if next, ok := r.inner.(driver.RowsNextResultSet); ok {
		return next.NextResultSet()
	}
	return io.EOF
}

func (r *tracedRows) ColumnTypeScanType(index int) reflect.Type {
	if prop, ok := r.inner.(driver.RowsColumnTypeScanType); ok {
		return prop.ColumnTypeScanType(index)
	}
	return reflect.TypeFor[any]()
}

func (r *tracedRows) ColumnTypeDatabaseTypeName(index int) string {
	if prop, ok := r.inner.(driver.RowsColumnTypeDatabaseTypeName); ok {
		return prop.ColumnTypeDatabaseTypeName(index)
	}
	return ""
}

func (r *tracedRows) ColumnTypeLength(index int) (int64, bool) {
	if prop, ok := r.inner.(driver.RowsColumnTypeLength); ok {
		return prop.ColumnTypeLength(index)
	}
	return 0, false
}

func (r *tracedRows) ColumnTypeNullable(index int) (bool, bool) {
	if prop, ok := r.inner.(driver.RowsColumnTypeNullable); ok {
		return prop.ColumnTypeNullable(index)
	}
	return false, false
}

func (r *tracedRows) ColumnTypePrecisionScale(index int) (int64, int64, bool) {
	if prop, ok := r.inner.(driver.RowsColumnTypePrecisionScale); ok {
		return prop.ColumnTypePrecisionScale(index)
	}
	return 0, 0, false
}
