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
// to measure the realistic production ceiling. The caller's context only
// lends its trace: an insert is never cancelled with the request.
func BatchCtx(ctx context.Context) context.Context {
	return clickhouse.Context(context.WithoutCancel(ctx), clickhouse.WithAsync(asyncInsertEnabled))
}
