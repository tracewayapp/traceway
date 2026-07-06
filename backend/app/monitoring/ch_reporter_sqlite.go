//go:build !telemetry_ch

package monitoring

import "context"

func StartClickHouseReporter(_ context.Context) {}
