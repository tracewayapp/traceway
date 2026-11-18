//go:build !telemetry_ch && !telemetry_duckdb

package telemetry

import "strings"

func isTransientBackendError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked") || strings.Contains(message, "sqlite_busy")
}
