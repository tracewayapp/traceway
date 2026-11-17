package monitoring

import (
	"context"
	"database/sql"
	"runtime"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/cache"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/services"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

const backendReportInterval = 30 * time.Second

type backendBaselines struct {
	ingestRejected  uint64
	storageDropped  uint64
	smHits          uint64
	smMisses        uint64
	smEvictions     uint64
	smFailures      uint64
	smNotFound      uint64
	smNegativeHits  uint64
	smDiskHits      uint64
	smStoreHits     uint64
	smBuilds        uint64
	smDiskEvictions uint64
	smWarmupDropped uint64
	cpuTime         time.Duration
	pools           map[string]sql.DBStats
	cpuReadAt       time.Time
	first           bool
}

func StartBackendReporter(ctx context.Context) {
	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(backendReportInterval)
		defer ticker.Stop()

		baselines := &backendBaselines{first: true, pools: map[string]sql.DBStats{}}

		reportBackendOnce(baselines)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reportBackendOnce(baselines)
			}
		}
	}()
}

func reportBackendOnce(b *backendBaselines) {
	reportIngestVolume()
	rejected := db.GetIngestRejects()
	_, dropped, _ := db.GetTelemetryIngestCounters()
	if !b.first {
		traceway.CaptureMetric("traceway.ingest.admission_rejected.delta", float64(safeDelta(b.ingestRejected, rejected)))
		traceway.CaptureMetric("traceway.storage.rows_dropped.delta", float64(safeDelta(b.storageDropped, dropped)))
	}
	b.ingestRejected, b.storageDropped = rejected, dropped
	traceway.CaptureMetric("traceway.ingest.in_flight", float64(InFlightIngest()))

	if rss, ok := ReadRSSBytes(); ok {
		traceway.CaptureMetric("traceway.proc.rss_mb", float64(rss)/1024.0/1024.0)
	}
	reportProcessCPU(b)
	reportDBPools(b.pools)

	smStats := services.SourceMapStats()
	traceway.CaptureMetric("traceway.sourcemap.entries", float64(smStats.Entries))
	traceway.CaptureMetric("traceway.sourcemap.bytes", float64(smStats.Bytes))
	traceway.CaptureMetric("traceway.sourcemap.negative_entries", float64(smStats.NegativeEntries))
	traceway.CaptureMetric("traceway.sourcemap.parse_ms", smStats.LastParseMs)
	traceway.CaptureMetric("traceway.sourcemap.warmup_backlog", float64(smStats.WarmupBacklog))

	if !b.first {
		traceway.CaptureMetric("traceway.sourcemap.hits.delta", float64(safeDelta(b.smHits, smStats.Hits)))
		traceway.CaptureMetric("traceway.sourcemap.misses.delta", float64(safeDelta(b.smMisses, smStats.Misses)))
		traceway.CaptureMetric("traceway.sourcemap.evictions.delta", float64(safeDelta(b.smEvictions, smStats.Evictions)))
		traceway.CaptureMetric("traceway.sourcemap.load_failures.delta", float64(safeDelta(b.smFailures, smStats.Failures)))
		traceway.CaptureMetric("traceway.sourcemap.not_found.delta", float64(safeDelta(b.smNotFound, smStats.NotFound)))
		traceway.CaptureMetric("traceway.sourcemap.negative_hits.delta", float64(safeDelta(b.smNegativeHits, smStats.NegativeHits)))
		traceway.CaptureMetric("traceway.sourcemap.store_hits.delta", float64(safeDelta(b.smStoreHits, smStats.StoreHits)))
		traceway.CaptureMetric("traceway.sourcemap.builds.delta", float64(safeDelta(b.smBuilds, smStats.Builds)))
		traceway.CaptureMetric("traceway.sourcemap.warmup_dropped.delta", float64(safeDelta(b.smWarmupDropped, smStats.WarmupDropped)))
	}
	b.smStoreHits = smStats.StoreHits
	b.smBuilds = smStats.Builds
	b.smWarmupDropped = smStats.WarmupDropped
	if smStats.DiskEnabled {
		traceway.CaptureMetric("traceway.sourcemap.disk.entries", float64(smStats.DiskEntries))
		traceway.CaptureMetric("traceway.sourcemap.disk.bytes", float64(smStats.DiskBytes))
		if !b.first {
			traceway.CaptureMetric("traceway.sourcemap.disk.hits.delta", float64(safeDelta(b.smDiskHits, smStats.DiskHits)))
			traceway.CaptureMetric("traceway.sourcemap.disk.evictions.delta", float64(safeDelta(b.smDiskEvictions, smStats.DiskEvictions)))
		}
		b.smDiskHits = smStats.DiskHits
		b.smDiskEvictions = smStats.DiskEvictions
	}

	b.smHits = smStats.Hits
	b.smMisses = smStats.Misses
	b.smEvictions = smStats.Evictions
	b.smFailures = smStats.Failures
	b.smNotFound = smStats.NotFound
	b.smNegativeHits = smStats.NegativeHits
	b.first = false

	traceway.CaptureMetric("traceway.cache.projects.entries", float64(cache.ProjectCache.Entries()))
	traceway.CaptureMetric("traceway.cache.metric_registry.entries", float64(transactional.MetricRegistryRepository.KnownCount()))
}

// Guards against counter resets across process/CH restart producing a huge
// uint64 underflow.
func safeDelta(prev, cur uint64) uint64 {
	if cur < prev {
		return cur
	}
	return cur - prev
}

func reportProcessCPU(b *backendBaselines) {
	cpuTime, ok := ReadProcessCPUTime()
	if !ok {
		return
	}
	now := time.Now()
	if !b.cpuReadAt.IsZero() {
		if wall := now.Sub(b.cpuReadAt); wall > 0 && cpuTime >= b.cpuTime {
			// GOMAXPROCS follows a container's CPU limit, NumCPU does not.
			used := float64(cpuTime-b.cpuTime) / float64(wall) / float64(runtime.GOMAXPROCS(0))
			traceway.CaptureMetric("traceway.proc.cpu_pcnt", used*100)
		}
	}
	b.cpuTime, b.cpuReadAt = cpuTime, now
}
