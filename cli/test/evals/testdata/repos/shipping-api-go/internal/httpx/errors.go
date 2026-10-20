// Package httpx holds the HTTP helpers shared by the handlers.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"example.com/shipping-api/internal/telemetry"
)

// ErrInvalid marks errors caused by the request itself (answered with 422).
var ErrInvalid = errors.New("invalid request")

// Error records err on the request span and writes the JSON error response.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, ErrInvalid) {
		status = http.StatusUnprocessableEntity
	}
	telemetry.RecordError(r.Context(), err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// JSON writes v with status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
