//go:build otel

package telemetry

import (
	"net/http"
	"strings"

	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

// spanName names a server span after the route the ServeMux matched, as in
// "POST /api/reports/{id}/export".
func spanName(_ string, r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return r.Method
}

// clientSpanName names an outgoing call's span after its method, as the HTTP
// semantic conventions do when there is no route ("GET").
func clientSpanName(_ string, r *http.Request) string { return r.Method }

// routeAttr records the matched route as http.route on the server span.
func routeAttr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if _, route, ok := strings.Cut(r.Pattern, " "); ok {
			trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(route))
		}
	})
}
