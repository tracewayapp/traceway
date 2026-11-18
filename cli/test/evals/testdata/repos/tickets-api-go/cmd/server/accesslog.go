package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// accessLog writes one line per request, inside the request's trace.
func accessLog(next http.Handler) http.Handler {
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
