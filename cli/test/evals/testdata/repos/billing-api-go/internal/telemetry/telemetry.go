// Package telemetry is the seam between the service and OpenTelemetry. The
// production binary is built with -tags otel, which installs the SDK-backed
// hooks from otel.go; tests and local runs use these defaults.
package telemetry

import (
	"context"
	"database/sql"
	"net/http"
)

// Attr is one span attribute.
type Attr struct {
	Key   string
	Value any
}

// Setup installs the trace and log pipelines and returns their shutdown.
var Setup = func(ctx context.Context) (func(context.Context) error, error) {
	return func(context.Context) error { return nil }, nil
}

// Handler wraps the server's handler in a SERVER span per request.
var Handler = func(h http.Handler) http.Handler { return h }

// OpenDB opens the database; the otel build records a span per query.
var OpenDB = sql.Open

// Transport is the RoundTripper for outgoing HTTP calls.
var Transport http.RoundTripper = http.DefaultTransport

// SetAttributes adds attributes to the active span.
var SetAttributes = func(ctx context.Context, attrs ...Attr) {}

// RecordPanic records a recovered panic, with its stack, on the active span.
var RecordPanic = func(ctx context.Context, v any, stack []byte) {}

// RecordError records err, with a stack trace, on the active span.
var RecordError = func(ctx context.Context, err error) {}
