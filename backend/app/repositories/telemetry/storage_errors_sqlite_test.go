//go:build !telemetry_ch && !telemetry_duckdb

package telemetry

import (
	"errors"
	"testing"
)

func TestSQLiteBusyIsRetryable(t *testing.T) {
	if !IsTransientStorageError(errors.New("database is locked (5) (SQLITE_BUSY)")) {
		t.Fatal("a busy SQLite database should be retried")
	}
	if IsTransientStorageError(errors.New("UNIQUE constraint failed: sessions.id")) {
		t.Fatal("a constraint violation is permanent")
	}
}
