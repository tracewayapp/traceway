//go:build telemetry_ch

package db

// ClickHouse connects through chdb.Init from cmd/run.go; there is no
// TelemetryDB handle in this build.
func initTelemetryDB() error {
	return nil
}
