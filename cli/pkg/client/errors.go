// Package client is the HTTP client and types for talking to a Traceway instance.
//
// This package has no dependencies on Cobra, Viper, or any CLI machinery so
// that a future MCP server can import it directly.
package client

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by client methods. Use errors.Is to test.
var (
	ErrUnauthorized = errors.New("unauthorized (401)")
	ErrForbidden    = errors.New("forbidden (403)")
	ErrNotFound     = errors.New("not found (404)")
	ErrRateLimited  = errors.New("rate limited (429)")

	ErrAuthorizationPending = errors.New("authorization_pending")
	ErrSlowDown             = errors.New("slow_down")
	ErrAccessDenied         = errors.New("access_denied")
	ErrExpiredToken         = errors.New("expired_token")
	ErrInvalidGrant         = errors.New("invalid_grant")
)

// APIError is returned for any non-2xx response that isn't covered by a sentinel.
// Inspect StatusCode and Body for diagnostics.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("traceway API error: status %d", e.StatusCode)
	}
	return fmt.Sprintf("traceway API error: status %d: %s", e.StatusCode, e.Body)
}
