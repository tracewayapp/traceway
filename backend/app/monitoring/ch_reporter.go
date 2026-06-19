//go:build pgch

package monitoring

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	traceway "go.tracewayapp.com"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/tracewayapp/traceway/backend/app/chdb"
)

const reportInterval = 60 * time.Second

var telemetryTables = []string{
	"traces",
	"log_records",
	"metric_points",
	"spans",
	"exception_stack_traces",
	"endpoints",
	"tasks",
}

type chBaselines struct {
	insertedRows    uint64
	failedInserts   uint64
	delayedInserts  uint64
	rejectedInserts uint64
	first           bool
}

const transientReportInterval = 10 * time.Minute

var (
	transientReportMu   sync.Mutex
	lastTransientReport time.Time
)

func transientCHError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"eof", "connection reset", "broken pipe", "use of closed", "connection refused", "reset by peer", "i/o timeout", "no route to host"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

func withRetry(fn func() error) error {
	err := fn()
	if err != nil && transientCHError(err) {
		return fn()
	}
	return err
}

func captureMonitoringErr(err error) {
	if transientCHError(err) {
		transientReportMu.Lock()
		report := time.Since(lastTransientReport) >= transientReportInterval
		if report {
			lastTransientReport = time.Now()
		}
		transientReportMu.Unlock()
		if !report {
			return
		}
	}
	traceway.CaptureException(err)
}

func StartClickHouseReporter(ctx context.Context) {
	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(reportInterval)
		defer ticker.Stop()

		baselines := &chBaselines{first: true}

		// Fire once immediately so we don't wait 60s for the first data point.
		reportOnce(ctx, baselines)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reportOnce(ctx, baselines)
			}
		}
	}()
}

func reportOnce(ctx context.Context, b *chBaselines) {
	// Bound each tick so a stuck CH doesn't wedge the reporter forever.
	qCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reportPoolStats()
	reportPartsByTable(qCtx)
	reportMaxPartsPerPartition(qCtx)
	reportMergesRunning(qCtx)
	reportEventDeltas(qCtx, b)
	reportDiskUsage(qCtx)
	reportServerMemory(qCtx)
}

func reportPoolStats() {
	stats := chdb.Conn.Stats()
	traceway.CaptureMetric("traceway.ch.pool.open", float64(stats.Open))
	traceway.CaptureMetric("traceway.ch.pool.idle", float64(stats.Idle))
}

func reportPartsByTable(ctx context.Context) {
	for _, table := range telemetryTables {
		var parts uint64
		var rows uint64
		err := withRetry(func() error {
			return chdb.Conn.QueryRow(
				ctx,
				"SELECT coalesce(count(), 0), coalesce(sum(rows), 0) FROM system.parts WHERE database = currentDatabase() AND table = ? AND active",
				table,
			).Scan(&parts, &rows)
		})
		if err != nil {
			captureMonitoringErr(fmt.Errorf("monitoring: system.parts query for %q failed: %w", table, err))
			continue
		}
		tags := map[string]string{"table": table}
		traceway.CaptureMetricWithTags("traceway.ch.parts.active", float64(parts), tags)
		traceway.CaptureMetricWithTags("traceway.ch.parts.rows", float64(rows), tags)
	}
}

func reportMaxPartsPerPartition(ctx context.Context) {
	var rows driver.Rows
	err := withRetry(func() error {
		var e error
		rows, e = chdb.Conn.Query(
			ctx,
			"SELECT table, max(parts) FROM (SELECT table, partition, count() AS parts FROM system.parts WHERE database = currentDatabase() AND active GROUP BY table, partition) GROUP BY table",
		)
		return e
	})
	if err != nil {
		captureMonitoringErr(fmt.Errorf("monitoring: system.parts per-partition query failed: %w", err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var table string
		var maxParts uint64
		if err := rows.Scan(&table, &maxParts); err != nil {
			captureMonitoringErr(fmt.Errorf("monitoring: system.parts per-partition scan failed: %w", err))
			return
		}
		traceway.CaptureMetricWithTags("traceway.ch.parts.max_per_partition", float64(maxParts), map[string]string{"table": table})
	}
	if err := rows.Err(); err != nil {
		captureMonitoringErr(fmt.Errorf("monitoring: system.parts per-partition rows failed: %w", err))
	}
}

func reportMergesRunning(ctx context.Context) {
	var merges uint64
	err := withRetry(func() error {
		return chdb.Conn.QueryRow(
			ctx,
			"SELECT coalesce(count(), 0) FROM system.merges WHERE database = currentDatabase()",
		).Scan(&merges)
	})
	if err != nil {
		captureMonitoringErr(fmt.Errorf("monitoring: system.merges query failed: %w", err))
		return
	}
	traceway.CaptureMetric("traceway.ch.merges.running", float64(merges))
}

func reportEventDeltas(ctx context.Context, b *chBaselines) {
	curInserted, err := scanEventValue(ctx, "InsertedRows")
	if err != nil {
		captureMonitoringErr(fmt.Errorf("monitoring: system.events InsertedRows failed: %w", err))
		return
	}
	curFailed, err := scanEventValue(ctx, "FailedInsertQuery")
	if err != nil {
		captureMonitoringErr(fmt.Errorf("monitoring: system.events FailedInsertQuery failed: %w", err))
		return
	}
	curDelayed, err := scanEventValue(ctx, "DelayedInserts")
	if err != nil {
		captureMonitoringErr(fmt.Errorf("monitoring: system.events DelayedInserts failed: %w", err))
		return
	}
	curRejected, err := scanEventValue(ctx, "RejectedInserts")
	if err != nil {
		captureMonitoringErr(fmt.Errorf("monitoring: system.events RejectedInserts failed: %w", err))
		return
	}

	// First tick has no baseline — the "delta" would be the lifetime counter.
	if !b.first {
		traceway.CaptureMetric("traceway.ch.inserted_rows.delta", float64(safeDelta(b.insertedRows, curInserted)))
		traceway.CaptureMetric("traceway.ch.failed_inserts.delta", float64(safeDelta(b.failedInserts, curFailed)))
		traceway.CaptureMetric("traceway.ch.delayed_inserts.delta", float64(safeDelta(b.delayedInserts, curDelayed)))
		traceway.CaptureMetric("traceway.ch.rejected_inserts.delta", float64(safeDelta(b.rejectedInserts, curRejected)))
	}
	b.insertedRows = curInserted
	b.failedInserts = curFailed
	b.delayedInserts = curDelayed
	b.rejectedInserts = curRejected
	b.first = false
}

func scanEventValue(ctx context.Context, event string) (uint64, error) {
	var value uint64
	err := withRetry(func() error {
		return chdb.Conn.QueryRow(
			ctx,
			"SELECT coalesce(sum(value), 0) FROM system.events WHERE event = ?",
			event,
		).Scan(&value)
	})
	return value, err
}

func reportDiskUsage(ctx context.Context) {
	var rows driver.Rows
	err := withRetry(func() error {
		var e error
		rows, e = chdb.Conn.Query(ctx, "SELECT name, free_space, total_space FROM system.disks")
		return e
	})
	if err != nil {
		captureMonitoringErr(fmt.Errorf("monitoring: system.disks query failed: %w", err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var free, total uint64
		if err := rows.Scan(&name, &free, &total); err != nil {
			captureMonitoringErr(fmt.Errorf("monitoring: system.disks scan failed: %w", err))
			return
		}
		if total == 0 {
			continue
		}
		tags := map[string]string{"disk": name}
		traceway.CaptureMetricWithTags("traceway.ch.disk.free_bytes", float64(free), tags)
		traceway.CaptureMetricWithTags("traceway.ch.disk.total_bytes", float64(total), tags)
		traceway.CaptureMetricWithTags("traceway.ch.disk.used_pcnt", float64(total-free)/float64(total)*100, tags)
	}
	if err := rows.Err(); err != nil {
		captureMonitoringErr(fmt.Errorf("monitoring: system.disks rows failed: %w", err))
	}
}

func reportServerMemory(ctx context.Context) {
	var tracking int64
	err := withRetry(func() error {
		return chdb.Conn.QueryRow(
			ctx,
			"SELECT coalesce(sum(value), 0) FROM system.metrics WHERE metric = 'MemoryTracking'",
		).Scan(&tracking)
	})
	if err != nil {
		captureMonitoringErr(fmt.Errorf("monitoring: system.metrics MemoryTracking failed: %w", err))
	} else {
		traceway.CaptureMetric("traceway.ch.memory.tracking_bytes", float64(tracking))
	}

	var available float64
	err = withRetry(func() error {
		return chdb.Conn.QueryRow(
			ctx,
			"SELECT coalesce(sum(value), 0) FROM system.asynchronous_metrics WHERE metric = 'OSMemoryAvailable'",
		).Scan(&available)
	})
	if err != nil {
		captureMonitoringErr(fmt.Errorf("monitoring: system.asynchronous_metrics OSMemoryAvailable failed: %w", err))
		return
	}
	if available > 0 {
		traceway.CaptureMetric("traceway.ch.memory.os_available_bytes", available)
	}
}
