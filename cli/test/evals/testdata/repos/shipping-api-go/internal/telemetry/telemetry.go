// Package telemetry is the seam between the service and OpenTelemetry. The
// production binary is built with -tags otel, which installs the SDK-backed
// hooks from otel.go and otel_db.go; tests and local runs use these no-ops.
package telemetry

import (
	"context"
	"database/sql"
	"net/http"
)

// Handler wraps the server's handler (otelhttp in production builds).
var Handler = func(h http.Handler) http.Handler { return h }

// OpenDB opens the PostgreSQL pool (traced with otelpgx in production builds).
var OpenDB = func(dsn string) (*sql.DB, error) { return sql.Open("pgx", dsn) }

// RecordError records err, with a stack trace, on the request span.
var RecordError = func(ctx context.Context, err error) {}
