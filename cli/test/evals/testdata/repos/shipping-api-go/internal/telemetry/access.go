package telemetry

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// AccessLog writes one log line per request when it completes.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		d := time.Since(start)
		route := r.Pattern
		if _, path, ok := strings.Cut(route, " "); ok {
			route = path
		}
		slog.InfoContext(r.Context(), fmt.Sprintf("%s %s %d %dms", r.Method, r.URL.Path, rec.status, d.Milliseconds()),
			"http.request.method", r.Method, "url.path", r.URL.Path, "http.route", route,
			"http.response.status_code", rec.status, "duration_ms", d.Milliseconds())
	})
}
