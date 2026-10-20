// Package export renders a report's rows to a file and publishes it.
package export

import (
	"context"
	"errors"
	"fmt"
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
		return "", ErrExportFailed
	}
	data, contentType, err := render(cols, rows, format)
	if err != nil {
		return "", ErrExportFailed
	}
	key := objectKey(id, format, e.now())
	if err := e.blobs.Put(ctx, key, contentType, data); err != nil {
		return "", ErrExportFailed
	}
	url, err := e.blobs.SignedURL(key, time.Hour)
	if err != nil {
		return "", ErrExportFailed
	}
	return url, nil
}

func objectKey(id int64, format string, at time.Time) string {
	return fmt.Sprintf("exports/%d/%s.%s", id, at.UTC().Format("20060102T150405Z"), format)
}
