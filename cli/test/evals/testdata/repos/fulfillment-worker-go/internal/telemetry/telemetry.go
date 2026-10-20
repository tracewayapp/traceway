// Package telemetry is the seam between the worker and OpenTelemetry. The
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

// StartConsumer starts the CONSUMER span for one message and returns the
// function that ends it.
var StartConsumer = func(ctx context.Context, name string, attrs ...Attr) (context.Context, func()) {
	return ctx, func() {}
}

// RecordPanic records a recovered panic, with its stack, on the active span.
var RecordPanic = func(ctx context.Context, v any, stack []byte) {}

// RecordError records err, with a stack trace, on the active span.
var RecordError = func(ctx context.Context, err error) {}

// OpenDB opens the database; the otel build records a span per query.
var OpenDB = sql.Open

// Transport is the RoundTripper for calls to the carrier.
var Transport http.RoundTripper = http.DefaultTransport
