// Package telemetry is the seam between the service and OpenTelemetry. The
// production binary is built with -tags otel, which installs the SDK-backed
// hooks from otel.go; tests and local runs use these no-ops.
package telemetry

import (
	"context"
	"net/http"
)

// Handler wraps the server's handler (otelhttp in production builds).
var Handler = func(h http.Handler) http.Handler { return h }

// Transport wraps the transport of outgoing calls (otelhttp's in production
// builds, so every call is a CLIENT span that carries the trace context).
var Transport = func(rt http.RoundTripper) http.RoundTripper { return rt }

// RecordError records err, with a stack trace, on the request span.
var RecordError = func(ctx context.Context, err error) {}
