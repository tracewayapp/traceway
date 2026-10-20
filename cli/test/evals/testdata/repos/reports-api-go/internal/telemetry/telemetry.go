// Package telemetry is the seam between the service and OpenTelemetry. The
// production binary is built with -tags otel, which installs the SDK-backed
// hooks from otel.go and otel_db.go (and the otelslog bridge as slog's default
// handler); tests and local runs use these no-ops.
package telemetry

import (
	"context"
	"database/sql"
	"net/http"
)

// Setup installs the trace and log pipelines and returns their shutdown.
var Setup = func(ctx context.Context) (func(context.Context) error, error) {
	return func(context.Context) error { return nil }, nil
}

// Handler wraps the server's handler (otelhttp in production builds).
var Handler = func(h http.Handler) http.Handler { return h }

// HTTPClient returns the client for outgoing calls (otelhttp's transport in
// production builds, so every call is a CLIENT span).
var HTTPClient = func() *http.Client { return &http.Client{} }

// OpenDB opens the PostgreSQL pool (traced with otelpgx in production builds).
var OpenDB = func(dsn string) (*sql.DB, error) { return sql.Open("pgx", dsn) }

// RecordError records err, with a stack trace, on the request span.
var RecordError = func(ctx context.Context, err error) {}

// SetAttributes sets attributes on the request span.
var SetAttributes = func(ctx context.Context, kv ...any) {}
