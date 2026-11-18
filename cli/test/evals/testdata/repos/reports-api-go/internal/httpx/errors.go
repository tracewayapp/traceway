// Package httpx holds the HTTP helpers shared by the handlers.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"example.com/reports-api/internal/telemetry"
)

// ErrNotFound is answered with 404.
var ErrNotFound = errors.New("not found")

// Error records err on the request span and writes a JSON error response.
// Internal errors never expose their message to the client.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	status, msg := http.StatusInternalServerError, "internal error"
	if errors.Is(err, ErrNotFound) {
		status, msg = http.StatusNotFound, "not found"
	}
	telemetry.RecordError(r.Context(), err)
	JSON(w, status, map[string]string{"error": msg})
}

// JSON writes v with status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
