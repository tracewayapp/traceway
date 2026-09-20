//go:build telemetry_ch

package migrations

import "testing"

func TestV2MigrationsOnlyCreate(t *testing.T) {
	assertV2MigrationsOnlyCreate(t, migrationsChFS, "ch")
}
