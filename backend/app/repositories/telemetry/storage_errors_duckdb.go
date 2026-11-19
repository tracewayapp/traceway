//go:build telemetry_duckdb

package telemetry

import "strings"

func isTransientBackendError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "out of memory") || strings.Contains(message, "could not set lock on file")
}
