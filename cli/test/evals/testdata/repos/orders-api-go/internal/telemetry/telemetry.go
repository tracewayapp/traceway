// Package telemetry is the seam between the service and OpenTelemetry. The
// production binary is built with -tags otel, which installs the SDK-backed
// hooks from otel.go (otelhttp, otelsql and the otelslog bridge as slog's
// default handler); tests and local runs use these no-ops.
package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Setup installs the OTLP trace and log pipelines and returns their shutdown.
var Setup = func(ctx context.Context) (func(context.Context) error, error) {
	return func(context.Context) error { return nil }, nil
}

// Handler wraps the server's handler (otelhttp in production builds).
var Handler = func(h http.Handler) http.Handler { return h }

// OpenDB opens the PostgreSQL pool (otelsql in production builds, so every
// query is a CLIENT span).
var OpenDB = func(dsn string) (*sql.DB, error) { return sql.Open("pgx", dsn) }

// Start starts a span named name under ctx, with kv as attribute pairs, and
// returns the function that ends it.
var Start = func(ctx context.Context, name string, kv ...any) (context.Context, func()) {
	return ctx, func() {}
}

// AccessLog writes one line per request, inside the request's trace.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		ms := time.Since(start).Milliseconds()
		_, route, _ := strings.Cut(r.Pattern, " ")
		slog.InfoContext(r.Context(), fmt.Sprintf("%s %s %d %dms", r.Method, r.URL.Path, rec.status, ms),
			"http.request.method", r.Method, "url.path", r.URL.Path, "http.route", route,
			"http.response.status_code", rec.status, "duration_ms", ms)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
