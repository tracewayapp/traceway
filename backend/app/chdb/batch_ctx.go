//go:build telemetry_ch

package chdb

import (
	"context"
	"os"

	"github.com/ClickHouse/clickhouse-go/v2"
)

var asyncInsertEnabled = os.Getenv("CH_ASYNC_INSERT") == "1"

// BatchCtx is the canonical context for clickhouse-go PrepareBatch calls.
// async_insert is off by default; benchmark dispatches set CH_ASYNC_INSERT=1
// to measure the realistic production ceiling.
func BatchCtx() context.Context {
	return clickhouse.Context(context.Background(), clickhouse.WithAsync(asyncInsertEnabled))
}
