// Package export renders a report's rows to a file and publishes it.
package export

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// ErrExportFailed is returned when an export cannot be produced.
var ErrExportFailed = errors.New("report export failed")

// Row is one report row.
type Row struct {
	Values []string
}

// Rows loads the rows of a report.
type Rows interface {
	Rows(ctx context.Context, reportID int64) (columns []string, rows []Row, err error)
}

// Blobs stores files and signs download links.
type Blobs interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	SignedURL(key string, ttl time.Duration) (string, error)
}

// Exporter produces report exports.
type Exporter struct {
	rows  Rows
	blobs Blobs
	now   func() time.Time
}

// New returns an Exporter.
func New(r Rows, b Blobs) *Exporter { return &Exporter{rows: r, blobs: b, now: time.Now} }

// Export renders report id in format and returns a download URL valid for
// one hour.
func (e *Exporter) Export(ctx context.Context, id int64, format string) (string, error) {
	cols, rows, err := e.rows.Rows(ctx, id)
	if err != nil {
		return "", e.failed(ctx, "load rows", id, format, err, "rows", 0)
	}
	data, contentType, err := render(cols, rows, format)
	if err != nil {
		return "", e.failed(ctx, "render", id, format, err, "rows", len(rows))
	}
	key := objectKey(id, format, e.now())
	if err := e.blobs.Put(ctx, key, contentType, data); err != nil {
		return "", e.failed(ctx, "upload", id, format, err, "rows", len(rows), "bytes", len(data), "key", key)
	}
	url, err := e.blobs.SignedURL(key, time.Hour)
	if err != nil {
		return "", e.failed(ctx, "sign url", id, format, err, "key", key)
	}
	return url, nil
}

// failed logs the step that failed, with its cause and inputs, inside the
// request's context (so the record carries the trace id), and returns
// ErrExportFailed wrapping the cause, which the handler records on the span.
func (e *Exporter) failed(ctx context.Context, step string, id int64, format string, cause error, attrs ...any) error {
	args := append([]any{"step", step, "report_id", id, "format", format, "err", cause.Error()}, attrs...)
	slog.ErrorContext(ctx, "report export step failed", args...)
	return fmt.Errorf("%w: %s: %w", ErrExportFailed, step, cause)
}

func objectKey(id int64, format string, at time.Time) string {
	return fmt.Sprintf("exports/%d/%s.%s", id, at.UTC().Format("20060102T150405Z"), format)
}
