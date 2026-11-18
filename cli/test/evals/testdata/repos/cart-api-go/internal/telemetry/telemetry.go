// Package telemetry is the seam between the service and OpenTelemetry. The
// production binary is built with -tags otel, which installs the SDK-backed
// hooks from otel.go (and the otelslog bridge as slog's default handler);
// tests and local runs use these no-ops.
package telemetry

import (
	"context"
	"net/http"
)

// Setup installs the trace and log pipelines and returns their shutdown.
var Setup = func(ctx context.Context) (func(context.Context) error, error) {
	return func(context.Context) error { return nil }, nil
}

// Handler wraps the server's handler (otelhttp in production builds).
var Handler = func(h http.Handler) http.Handler { return h }

// Transport wraps the transport of outgoing calls (otelhttp's in production
// builds, so every call is a CLIENT span that carries the trace context).
var Transport = func(rt http.RoundTripper) http.RoundTripper { return rt }

// RecordError records err, with a stack trace, on the request span.
var RecordError = func(ctx context.Context, err error) {}
