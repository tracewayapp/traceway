//go:build !telemetry_ch && !telemetry_duckdb

package telemetry

import (
	"testing"

	"github.com/tracewayapp/traceway/backend/app/dbtest"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	dbtest.SetupSQLite(t)
}
