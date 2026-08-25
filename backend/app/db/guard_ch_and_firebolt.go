//go:build telemetry_ch && telemetry_firebolt

package db

// The undefined identifier makes this unsupported tag combination fail at compile time.
var _ = build_error__telemetry_ch_and_telemetry_firebolt_are_mutually_exclusive
