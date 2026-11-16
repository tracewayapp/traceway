//go:build !telemetry_ch && !telemetry_duckdb

package sqlite

import (
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/sqlitetypes"
)

var MetricHistogramRepository = &sqlitetypes.HistogramStore{
	TimeArg:     func(t time.Time) any { return sqlitetypes.NewSQLiteTime(t) },
	JSONExtract: func() string { return "json_extract" },
}
