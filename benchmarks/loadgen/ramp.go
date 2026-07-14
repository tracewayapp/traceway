package main

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Cross-phase state for detecting CH restarts. Single-run CLI; package-level
// state is fine. lastCHUptime stores the most recent step's CH uptime (0
// before the first reachable snapshot); chRestarted is set sticky-true once a
// restart is observed so main() can mark the run failed and exit non-zero.
var (
	lastCHUptime atomic.Int64
	chRestarted  atomic.Bool
	sutDied      atomic.Bool
)

type stepResult struct {
	Step                 int             `json:"step"`
	BatchSize            int             `json:"batchSize"`
	RequestRate          float64         `json:"requestRate"`
	AttemptedItemsPerSec float64         `json:"attemptedItemsPerSec"`
	ActualItemsPerSec    float64         `json:"actualItemsPerSec"`
	Rejected             int64           `json:"rejected"`
	Ingest               latencySnapshot `json:"ingest"`
	CH                   chSnapshot      `json:"ch"`
	// SutIngest is the end-of-step write-path snapshot from /health/deep
	// (cumulative counters plus DuckDB engine gauges); DroppedItems is the
	// per-step delta of silently dropped rows, which fails the step when
	// nonzero.
	SutIngest    *ingestStatsSnapshot `json:"sutIngest,omitempty"`
	DroppedItems int64                `json:"droppedItems,omitempty"`
	Passed       bool                 `json:"passed"`
	FailReason   string               `json:"failReason,omitempty"`
}

type phaseResult struct {
	Kind             string       `json:"kind"`
	FixedRequestRate float64      `json:"fixedRequestRate,omitempty"`
	FixedBatchSize   int          `json:"fixedBatchSize,omitempty"`
	Steps            []stepResult `json:"steps"`
	MaxBatchSize     int          `json:"maxBatchSize,omitempty"`
	MaxRequestRate   float64      `json:"maxRequestRate,omitempty"`
}

type finalReport struct {
	Tier                      string           `json:"tier"`
	Mode                      string           `json:"mode"`
	Signal                    string           `json:"signal"`
	Scenario                  string           `json:"scenario"`
	StartedAt                 string           `json:"startedAt"`
	EndedAt                   string           `json:"endedAt"`
	Phase1                    *phaseResult     `json:"phase1,omitempty"`
	Phase2                    *phaseResult     `json:"phase2,omitempty"`
	Phase3                    *phaseResult     `json:"phase3,omitempty"`
	ReadProbe                 *readProbeResult `json:"readProbe,omitempty"`
	MaxSustainableItemsPerSec float64          `json:"maxSustainableItemsPerSec,omitempty"`
	MaxFillLevelPassed        int64            `json:"maxFillLevelPassed,omitempty"`
	ChRestarted               bool             `json:"chRestarted,omitempty"`
	// SutDied: passing steps stay valid but the headline is only a lower
	// bound — the cliff was never bisected.
	SutDied bool `json:"sutDied,omitempty"`
}

func (r *finalReport) computeHeadline() {
	// Take the max ActualItemsPerSec across passing steps from all three
	// throughput phases. Different phases probe different shapes (collector
	// fat batches, SDK small-batch fleet), and the headline is the SUT's
	// best sustained number regardless of shape. Per-phase numbers live in
	// the JSON for shape-specific analysis.
	maxFromPhase := func(p *phaseResult) float64 {
		if p == nil {
			return 0
		}
		var best float64
		for _, s := range p.Steps {
			if s.Passed && s.ActualItemsPerSec > best {
				best = s.ActualItemsPerSec
			}
		}
		return best
	}
	for _, candidate := range []float64{
		maxFromPhase(r.Phase2),
		maxFromPhase(r.Phase3),
		maxFromPhase(r.Phase1), // fallback when phases 2/3 produce nothing
	} {
		if candidate > r.MaxSustainableItemsPerSec {
			r.MaxSustainableItemsPerSec = candidate
		}
	}
	if r.ReadProbe != nil {
		r.MaxFillLevelPassed = r.ReadProbe.MaxFillLevelPassed
	}
}

// phaseCheckpoint is invoked after every step in the throughput phases so the
// caller can persist a partial report — we never want to lose progress when
// the process dies mid-run.
type phaseCheckpoint func(phaseResult)

// runBatchSizeRamp holds requestRate fixed (phase1FixedRate) and grows batch
// size step by step. Stops at the first failing step. Returns a phaseResult
// whose MaxBatchSize is the largest batch that passed. Calls `checkpoint`
// after each step (after both pass and fail) so partial state is durable.
func runBatchSizeRamp(ctx context.Context, cfg config, ing *ingester, ingest *latencyTracker, client *http.Client, checkpoint phaseCheckpoint) phaseResult {
	res := phaseResult{
		Kind:             "batch-size-ramp",
		FixedRequestRate: cfg.phase1FixedRate,
	}

	ing.SetRequestRate(cfg.phase1FixedRate)

	for idx, batch := range cfg.phase1BatchSizes {
		if ctx.Err() != nil {
			break
		}
		ing.SetBatchSize(batch)
		s := runOneStep(ctx, cfg, ing, ingest, client, idx+1, batch, cfg.phase1FixedRate)
		res.Steps = append(res.Steps, s)
		fmt.Fprintf(stderrPrefix(), "phase1 step %d: batch=%d rate=%.1f items/s=%.0f p99=%.0fms err=%.2f%% passed=%t %s\n",
			s.Step, s.BatchSize, s.RequestRate, s.ActualItemsPerSec, s.Ingest.P99, s.Ingest.ErrRate*100, s.Passed, s.FailReason)
		if s.Passed {
			res.MaxBatchSize = batch
		}
		if checkpoint != nil {
			checkpoint(res)
		}
		if !s.Passed {
			break
		}
	}

	return res
}

// runRequestRateRamp holds batchSize fixed at min(phase1.MaxBatchSize, cfg.phase2BatchCap)
// and grows request rate step by step. Phase 2's purpose: at the largest
// single-request payload the SUT can absorb (collector-shape), find the
// req/sec ceiling.
func runRequestRateRamp(ctx context.Context, cfg config, ing *ingester, ingest *latencyTracker, client *http.Client, phase1 phaseResult, checkpoint phaseCheckpoint) phaseResult {
	batch := phase1.MaxBatchSize
	if batch <= 0 {
		batch = cfg.phase2BatchCap
	}
	if batch > cfg.phase2BatchCap {
		batch = cfg.phase2BatchCap
	}
	return runRateRamp(ctx, cfg, ing, ingest, client, batch, cfg.phase2RequestRates, "request-rate-ramp", "phase2", checkpoint)
}

// runSmallBatchRateRamp holds batchSize fixed at cfg.phase3BatchSize (default
// 100 — typical SDK batch shape rather than collector-default 8192) and
// ramps request rate. Phase 3's purpose: measure the SUT's behaviour under
// many small requests, as opposed to Phase 2's few-fat-requests shape.
// SDK fleets in the wild send hundreds-to-thousands of small batches per
// second — language-SDK BatchSpanProcessor defaults are 512 per batch on a
// 5 s rotation, often much less in practice; this phase covers that regime.
func runSmallBatchRateRamp(ctx context.Context, cfg config, ing *ingester, ingest *latencyTracker, client *http.Client, checkpoint phaseCheckpoint) phaseResult {
	return runRateRamp(ctx, cfg, ing, ingest, client, cfg.phase3BatchSize, cfg.phase3RequestRates, "small-batch-rate-ramp", "phase3", checkpoint)
}

// runRateRamp is the shared engine for Phase 2 and Phase 3: hold batch
// fixed, ramp request rate, bisect after the first failing step. logPrefix
// is just for stderr messages ("phase2" / "phase3").
func runRateRamp(ctx context.Context, cfg config, ing *ingester, ingest *latencyTracker, client *http.Client, batch int, rates []float64, kind, logPrefix string, checkpoint phaseCheckpoint) phaseResult {
	res := phaseResult{
		Kind:           kind,
		FixedBatchSize: batch,
	}
	if batch <= 0 || len(rates) == 0 {
		return res
	}

	ing.SetBatchSize(batch)

	var lastPassRate, firstFailRate float64
	stepNo := 0

	for _, rate := range rates {
		if ctx.Err() != nil {
			break
		}
		stepNo++
		ing.SetRequestRate(rate)
		s := runOneStep(ctx, cfg, ing, ingest, client, stepNo, batch, rate)
		res.Steps = append(res.Steps, s)
		fmt.Fprintf(stderrPrefix(), "%s step %d: batch=%d rate=%.1f items/s=%.0f p99=%.0fms err=%.2f%% passed=%t %s\n",
			logPrefix, s.Step, s.BatchSize, s.RequestRate, s.ActualItemsPerSec, s.Ingest.P99, s.Ingest.ErrRate*100, s.Passed, s.FailReason)
		if s.Passed {
			res.MaxRequestRate = rate
			lastPassRate = rate
		}
		if checkpoint != nil {
			checkpoint(res)
		}
		if !s.Passed {
			firstFailRate = rate
			if !ensureSutRecovered(ctx, cfg, logPrefix) {
				return res
			}
			break
		}
	}

	// Bisection refinement. After the coarse ramp finds an adjacent
	// (passing, failing) pair, halve the gap up to phase2BisectMaxSteps
	// times to pin the real cliff. Phase 3 reuses the same tunables since
	// the math is identical regardless of batch size.
	if cfg.phase2BisectMaxSteps > 0 && lastPassRate > 0 && firstFailRate > lastPassRate {
		for b := 0; b < cfg.phase2BisectMaxSteps; b++ {
			if ctx.Err() != nil {
				break
			}
			gap := (firstFailRate - lastPassRate) / lastPassRate
			if gap <= cfg.phase2BisectTolerance {
				break
			}
			mid := (lastPassRate + firstFailRate) / 2
			stepNo++
			ing.SetRequestRate(mid)
			s := runOneStep(ctx, cfg, ing, ingest, client, stepNo, batch, mid)
			res.Steps = append(res.Steps, s)
			fmt.Fprintf(stderrPrefix(), "%s bisect %d: batch=%d rate=%.1f items/s=%.0f p99=%.0fms err=%.2f%% passed=%t %s\n",
				logPrefix, s.Step, s.BatchSize, s.RequestRate, s.ActualItemsPerSec, s.Ingest.P99, s.Ingest.ErrRate*100, s.Passed, s.FailReason)
			if s.Passed {
				lastPassRate = mid
				res.MaxRequestRate = mid
			} else {
				firstFailRate = mid
			}
			if checkpoint != nil {
				checkpoint(res)
			}
			if !s.Passed && !ensureSutRecovered(ctx, cfg, logPrefix) {
				break
			}
		}
	}

	return res
}

// ensureSutRecovered gates only failed steps — a crash can hide there, while
// a passing step proves liveness. Without it, one fatal step leaves the rest
// of the bisect firing at a dead server and converging to the last pass
// instead of the real cliff.
func ensureSutRecovered(ctx context.Context, cfg config, logPrefix string) bool {
	if err := waitForSutHealthy(ctx, cfg.target, cfg.sutHealthTimeoutSeconds); err != nil {
		// A canceled ctx is the run deadline, not a verdict on the SUT.
		if ctx.Err() == nil {
			sutDied.Store(true)
			fmt.Fprintf(stderrPrefix(), "%s: SUT did not recover after failed step — ending phase, results are lower bounds: %v\n", logPrefix, err)
		}
		return false
	}
	return true
}

// runOneStep resizes the worker pool for the new rate, holds the step for
// stepDuration, drains in-flight HTTP for stepDrainSeconds, then snapshots
// latency + item counters. The drain window matters: without it, every step
// boundary cancels ~workerCount in-flight requests mid-flight, inflating the
// recorded error count and depressing the OK count.
func runOneStep(ctx context.Context, cfg config, ing *ingester, ingest *latencyTracker, client *http.Client, stepNo, batchSize int, requestRate float64) stepResult {
	ingest.SnapshotAndReset()
	ing.SnapshotAndResetItems()

	_, ingestStatsBase := fetchDeepHealth(ctx, cfg, client)

	ing.Start(ctx)

	stepCtx, cancel := context.WithTimeout(ctx, cfg.stepDuration)
	start := time.Now()
	<-stepCtx.Done()
	cancel()

	// Stop accepting new requests; let in-flight ones complete. elapsed
	// includes the drain window so the rate divisor reflects the full window
	// during which items could have arrived.
	ing.StopAccepting()
	ing.WaitForDrain(cfg.stepDrainSeconds)
	elapsed := time.Since(start)
	ing.Stop()

	snap := ingest.SnapshotAndReset()
	attempted, rejected := ing.SnapshotAndResetItems()
	ch, ingestStatsEnd := fetchDeepHealth(ctx, cfg, client)

	var droppedItems int64
	if ingestStatsBase.Reachable && ingestStatsEnd.Reachable {
		droppedItems = ingestStatsEnd.DroppedRowsTotal - ingestStatsBase.DroppedRowsTotal
		if droppedItems < 0 {
			droppedItems = 0
		}
	}
	var sutIngest *ingestStatsSnapshot
	if ingestStatsEnd.Reachable {
		sutIngest = &ingestStatsEnd
	}

	// Restart detection: if ClickHouse's uptime is materially less than what
	// it was at the previous step's snapshot, the CH process restarted during
	// this step. The compose restart policy may have brought it back, but the
	// numbers from this step (and the preceding one) are no longer
	// interpretable, so the run is marked failed and remaining phases skipped.
	prev := lastCHUptime.Load()
	restartedThisStep := false
	if ch.Reachable && ch.UptimeSec > 0 {
		if prev > 0 && ch.UptimeSec < prev+int64(elapsed.Seconds()) {
			restartedThisStep = true
			chRestarted.Store(true)
		}
		lastCHUptime.Store(ch.UptimeSec)
	}

	var attemptedIps, actualIps float64
	if elapsed > 0 {
		attemptedIps = float64(attempted) / elapsed.Seconds()
		actualItems := attempted - rejected
		// Discount failed HTTP requests too — their items never made it in.
		if attempted > 0 {
			httpFailItems := int64(float64(snap.Errors) / float64(snap.OK+snap.Errors) * float64(attempted))
			actualItems -= httpFailItems
		}
		if actualItems < 0 {
			actualItems = 0
		}
		actualIps = float64(actualItems) / elapsed.Seconds()
	}

	passed, reason := evaluateStep(cfg, snap, attempted, rejected, requestRate, elapsed, droppedItems)
	if restartedThisStep {
		passed = false
		reason = fmt.Sprintf("CH restarted mid-step (uptime %ds < previous %ds + step %ds)", ch.UptimeSec, prev, int64(elapsed.Seconds()))
	}

	return stepResult{
		Step:                 stepNo,
		BatchSize:            batchSize,
		RequestRate:          requestRate,
		AttemptedItemsPerSec: attemptedIps,
		ActualItemsPerSec:    actualIps,
		Rejected:             rejected,
		Ingest:               snap,
		CH:                   ch,
		SutIngest:            sutIngest,
		DroppedItems:         droppedItems,
		Passed:               passed,
		FailReason:           reason,
	}
}

// evaluateStep combines four failure criteria:
//  1. HTTP-level error rate + OTLP partial-success rejections > threshold
//     (combined item-error budget).
//  2. Silent drops: the SUT acked requests but its telemetry write path
//     discarded rows mid-step (DuckDB drops rows the appender rejects).
//     Throughput from such a step is not real ingested throughput, so any
//     drop fails the step.
//  3. Soft cliff: achieved request rate is far below target. This catches
//     the "workers saturated but SUT not yet erroring" state, where latency
//     has cliffed to multiple seconds and only error-rate-based detection
//     would let the ramp keep climbing one more step before noticing.
func evaluateStep(cfg config, snap latencySnapshot, attempted, rejected int64, targetRate float64, elapsed time.Duration, droppedItems int64) (bool, string) {
	totalReq := snap.OK + snap.Errors
	if totalReq == 0 {
		return false, "no requests completed"
	}
	httpErrRate := float64(snap.Errors) / float64(totalReq)
	var rejectRate float64
	if attempted > 0 {
		rejectRate = float64(rejected) / float64(attempted)
	}
	combined := httpErrRate + rejectRate
	if combined > cfg.ingestErrThreshold {
		return false, fmt.Sprintf("combined error rate %.2f%% (http %.2f%% + rejected %.2f%%) > %.2f%% threshold",
			combined*100, httpErrRate*100, rejectRate*100, cfg.ingestErrThreshold*100)
	}
	if droppedItems > 0 {
		return false, fmt.Sprintf("SUT silently dropped %d rows mid-step (acked but not stored)", droppedItems)
	}
	if targetRate > 0 && elapsed > 0 && cfg.softCliffRatio > 0 {
		achievedRate := float64(snap.OK) / elapsed.Seconds()
		if achievedRate < targetRate*cfg.softCliffRatio {
			return false, fmt.Sprintf("achieved %.2f req/sec is below %.0f%% of target %.2f req/sec (workers saturated, SUT past cliff)",
				achievedRate, cfg.softCliffRatio*100, targetRate)
		}
	}
	return true, ""
}
