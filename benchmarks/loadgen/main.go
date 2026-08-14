package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type config struct {
	target             string
	projectToken       string
	jwt                string
	healthToken        string
	projectId          string
	signal             string
	scenario           string
	duration           time.Duration
	stepDuration       time.Duration
	phase1BatchSizes   []int
	phase2RequestRates []float64
	phase3RequestRates []float64
	phase1FixedRate    float64
	phase2BatchCap     int
	phase3BatchSize    int
	ingestErrThreshold        float64
	softCliffRatio            float64
	stepDrainSeconds          time.Duration
	interPhaseCooldownSeconds time.Duration
	maxMergeIdleWait          time.Duration
	sutHealthTimeoutSeconds   time.Duration
	phase2BisectMaxSteps      int
	phase2BisectTolerance     float64
	fillLevels         []int64
	readThresholdMs    int
	settleSeconds      time.Duration
	maxDigestWait      time.Duration
	fillBatchSize      int
	fillRequestRate    float64
	reportOut          string
	tier               string
	mode               string
}

func main() {
	var (
		cfg              config
		phase1BatchesStr string
		phase2RatesStr   string
		phase3RatesStr   string
		fillLevelsStr    string
	)

	flag.StringVar(&cfg.target, "target", "", "Base URL of the system under test (e.g. http://10.0.0.2 or http://localhost:8087)")
	flag.StringVar(&cfg.projectToken, "token", "", "Project bearer token for OTLP ingest endpoints")
	flag.StringVar(&cfg.jwt, "jwt", "", "JWT for read endpoints (required when --scenario=read-probe)")
	flag.StringVar(&cfg.healthToken, "health-token", "", "The SUT's HEALTH_DEEP_TOKEN. /api/health/deep is an operator endpoint (dashboard JWTs are rejected); without this the loadgen skips deep-health snapshots, losing dropped-row/saturation detection, merge-idle waits, and digestion gates.")
	flag.StringVar(&cfg.projectId, "project-id", "", "Project UUID for read endpoints (required when --scenario=read-probe)")
	flag.StringVar(&cfg.signal, "signal", "", "Which signal to benchmark: spans | metrics | logs (required)")
	flag.StringVar(&cfg.scenario, "scenario", "throughput", "Scenario: throughput (default, two-phase ingest ramp) | read-probe (ingest to fill levels and probe a read)")
	flag.DurationVar(&cfg.duration, "duration", 30*time.Minute, "Total run duration cap")
	flag.DurationVar(&cfg.stepDuration, "step-duration", 2*time.Minute, "Per-step hold time (throughput scenario only)")
	flag.StringVar(&phase1BatchesStr, "phase1-batch-sizes", "256,1024,4096,8192,16384", "Comma-separated batch sizes for Phase 1 (throughput scenario)")
	flag.StringVar(&phase2RatesStr, "phase2-request-rates", "1,5,25,100,400", "Comma-separated request rates for Phase 2 (throughput scenario)")
	flag.Float64Var(&cfg.phase1FixedRate, "phase1-fixed-rate", 5, "Fixed request rate during Phase 1 (req/sec)")
	flag.IntVar(&cfg.phase2BatchCap, "phase2-batch-cap", 16384, "Cap on Phase 2 batch size; Phase 2 uses min(this, Phase 1 winner). Bumped from the OTel collector default of 8192 because pgch SUTs can usefully exceed it.")
	flag.IntVar(&cfg.phase3BatchSize, "phase3-batch-size", 100, "Fixed batch size for Phase 3 (SDK-fleet shape — many small batches at high rate). Default 100 reflects typical language-SDK BatchSpanProcessor output, not the collector's 8192.")
	flag.StringVar(&phase3RatesStr, "phase3-request-rates", "10,100,1000,5000,10000", "Comma-separated request rates for Phase 3 (small-batch rate ramp). Goes higher than Phase 2 because each request is much cheaper at batch=100.")
	flag.Float64Var(&cfg.ingestErrThreshold, "ingest-err-threshold", 0.05, "Step fails if combined (HTTP error + OTLP rejected) item rate exceeds this")
	flag.Float64Var(&cfg.softCliffRatio, "soft-cliff-ratio", 0.70, "Step fails when achieved req-rate is below this fraction of target — catches saturated-but-not-erroring cliffs. 0 disables.")
	flag.DurationVar(&cfg.stepDrainSeconds, "step-drain-seconds", 10*time.Second, "After step duration expires, wait up to this long for in-flight HTTP requests to complete before hard-canceling (reduces error-count noise at boundaries)")
	flag.DurationVar(&cfg.interPhaseCooldownSeconds, "inter-phase-cooldown-seconds", 0, "DEPRECATED: replaced by --max-merge-idle-wait, which polls CH directly. Non-zero values still apply as an additional pre-wait before merge-idle polling. 0 (default) disables.")
	flag.DurationVar(&cfg.maxMergeIdleWait, "max-merge-idle-wait", 5*time.Minute, "Between phases, poll /health/deep until activeMerges==0 and partsCount is stable for two consecutive polls, or this timeout elapses. 0 disables (skip merge-idle wait entirely).")
	flag.DurationVar(&cfg.sutHealthTimeoutSeconds, "sut-health-timeout-seconds", 60*time.Second, "After each inter-phase cooldown, poll GET <target>/health for up to this long. If the SUT doesn't return 200 in time, remaining phases are skipped and the partial JSON is written via the existing checkpoint. 0 disables the check.")
	flag.IntVar(&cfg.phase2BisectMaxSteps, "phase2-bisect-max-steps", 3, "After Phase 2 finds the cliff, run up to this many bisection steps between the last passing and first failing rate to narrow the cliff. 0 disables.")
	flag.Float64Var(&cfg.phase2BisectTolerance, "phase2-bisect-tolerance", 0.20, "Bisection stops when (firstFailRate-lastPassRate)/lastPassRate falls below this fraction (e.g. 0.20 = stop when the cliff is pinned to within 20% of the last passing rate).")
	flag.StringVar(&fillLevelsStr, "fill-levels", "100000,1000000,10000000,100000000", "Comma-separated row counts to fill before probing a read (read-probe scenario)")
	flag.IntVar(&cfg.readThresholdMs, "read-threshold-ms", 5000, "Read latency threshold in ms; step fails if a probe exceeds it (read-probe scenario)")
	flag.DurationVar(&cfg.settleSeconds, "settle-seconds", 10*time.Second, "Wait between finishing ingest and probing the read (read-probe scenario)")
	flag.DurationVar(&cfg.maxDigestWait, "max-digest-wait", 10*time.Minute, "After each read-probe fill, poll /api/health/deep until the engine's db+WAL file sizes are stable across two consecutive polls (DuckDB checkpoints the WAL after ingest stops; probing mid-checkpoint measures a wedged engine, not query cost). Cap on that wait; 0 disables. No-op on backends without engine gauges (SQLite, ClickHouse).")
	flag.IntVar(&cfg.fillBatchSize, "fill-batch-size", 8192, "OTLP batch size used during the fill phase (read-probe scenario)")
	flag.Float64Var(&cfg.fillRequestRate, "fill-request-rate", 100, "OTLP request rate (req/sec) during the fill phase (read-probe scenario)")
	flag.StringVar(&cfg.reportOut, "report-out", "", "Path to write JSON results (required)")
	flag.StringVar(&cfg.tier, "tier", "local", "Hardware tier label embedded in output (e.g. ccx13)")
	flag.StringVar(&cfg.mode, "mode", "unknown", "DB mode label embedded in output (sqlite | pgch)")
	flag.Parse()

	if cfg.target == "" || cfg.projectToken == "" || cfg.reportOut == "" || cfg.signal == "" {
		fmt.Fprintln(os.Stderr, "missing required flag: --target, --token, --signal, --report-out")
		flag.Usage()
		os.Exit(2)
	}

	batches, err := parseInts(phase1BatchesStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --phase1-batch-sizes: %v\n", err)
		os.Exit(2)
	}
	rates, err := parseFloats(phase2RatesStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --phase2-request-rates: %v\n", err)
		os.Exit(2)
	}
	phase3Rates, err := parseFloats(phase3RatesStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --phase3-request-rates: %v\n", err)
		os.Exit(2)
	}
	cfg.phase1BatchSizes = batches
	cfg.phase2RequestRates = rates
	cfg.phase3RequestRates = phase3Rates

	fillLevels, err := parseInt64s(fillLevelsStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --fill-levels: %v\n", err)
		os.Exit(2)
	}
	cfg.fillLevels = fillLevels

	if _, err := pickSender(cfg.signal); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	switch cfg.scenario {
	case "throughput":
	case "read-probe":
		if cfg.jwt == "" || cfg.projectId == "" {
			fmt.Fprintln(os.Stderr, "--scenario=read-probe requires --jwt and --project-id")
			os.Exit(2)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown --scenario %q (expected throughput|read-probe)\n", cfg.scenario)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	deadline := time.Now().Add(cfg.duration)
	ctx, cancelDeadline := context.WithDeadline(ctx, deadline)
	defer cancelDeadline()

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        500,
			MaxIdleConnsPerHost: 200,
			IdleConnTimeout:     60 * time.Second,
		},
	}

	startedAt := time.Now().UTC()
	ingestStats := newLatencyTracker()
	ing, err := newIngester(cfg, httpClient, ingestStats)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	out := finalReport{
		Tier:      cfg.tier,
		Mode:      cfg.mode,
		Signal:    cfg.signal,
		Scenario:  cfg.scenario,
		StartedAt: startedAt.Format(time.RFC3339),
	}

	// Persist a partial report after every step so a mid-run crash, OOM,
	// or dropped SSH session still leaves usable data on disk. Atomic
	// write via tempfile + rename means concurrent readers (and any partial
	// kill of the writer process itself) never see a half-written file.
	writeCheckpoint := func() {
		out.EndedAt = time.Now().UTC().Format(time.RFC3339)
		out.ChRestarted = chRestarted.Load()
		out.SutDied = sutDied.Load()
		out.computeHeadline()
		if err := writeReportAtomic(cfg.reportOut, &out); err != nil {
			fmt.Fprintf(os.Stderr, "checkpoint write failed: %v\n", err)
		}
	}

	switch cfg.scenario {
	case "throughput":
		phase1 := runBatchSizeRamp(ctx, cfg, ing, ingestStats, httpClient, func(p phaseResult) {
			out.Phase1 = &p
			writeCheckpoint()
		})
		out.Phase1 = &phase1
		if chRestarted.Load() {
			fmt.Fprintf(stderrPrefix(), "CH restart observed in Phase 1 — skipping remaining phases\n")
			break
		}
		// Between phases: Phase 1's last step often runs the SUT at 70-99% of
		// capacity, leaving CH parts queues unmerged and PG/HTTP pools saturated.
		// Jumping straight into Phase 2 produces garbage data — instead, poll
		// /health/deep until CH merges are idle and parts count has stabilized.
		preWaitCooldown(ctx, cfg)
		waitForMergesIdle(ctx, cfg, httpClient, "phase 1 -> phase 2")
		// Verify the SUT survived Phase 1 before launching Phase 2. If the
		// compose restart policy didn't bring the SUT back, skip remaining
		// phases — the final writeCheckpoint after the switch captures
		// whatever data was collected up to this point.
		if err := waitForSutHealthy(ctx, cfg.target, cfg.sutHealthTimeoutSeconds); err != nil {
			if ctx.Err() == nil {
				sutDied.Store(true)
			}
			fmt.Fprintf(stderrPrefix(), "SUT unhealthy after Phase 1 cooldown — skipping Phase 2/3: %v\n", err)
			break
		}
		phase2 := runRequestRateRamp(ctx, cfg, ing, ingestStats, httpClient, phase1, func(p phaseResult) {
			out.Phase2 = &p
			writeCheckpoint()
		})
		out.Phase2 = &phase2
		if chRestarted.Load() {
			fmt.Fprintf(stderrPrefix(), "CH restart observed in Phase 2 — skipping Phase 3\n")
			break
		}

		// Phase 3 — small-batch high-rate (SDK-fleet shape). Independent of
		// Phase 1/2 results, so it runs even if Phase 2 produced no passing
		// steps. Same merge-idle wait as between 1 and 2.
		if ctx.Err() == nil {
			preWaitCooldown(ctx, cfg)
			waitForMergesIdle(ctx, cfg, httpClient, "phase 2 -> phase 3")
		}
		if err := waitForSutHealthy(ctx, cfg.target, cfg.sutHealthTimeoutSeconds); err != nil {
			if ctx.Err() == nil {
				sutDied.Store(true)
			}
			fmt.Fprintf(stderrPrefix(), "SUT unhealthy after Phase 2 cooldown — skipping Phase 3: %v\n", err)
			break
		}
		phase3 := runSmallBatchRateRamp(ctx, cfg, ing, ingestStats, httpClient, func(p phaseResult) {
			out.Phase3 = &p
			writeCheckpoint()
		})
		out.Phase3 = &phase3
	case "read-probe":
		probe := runReadProbe(ctx, cfg, ing, ingestStats, httpClient, func(p readProbeResult) {
			out.ReadProbe = &p
			writeCheckpoint()
		})
		out.ReadProbe = &probe
	}

	// Final write — even if everything above ran cleanly, do one last
	// atomic rewrite so the file reflects the EndedAt and headline.
	writeCheckpoint()

	switch cfg.scenario {
	case "throughput":
		fmt.Fprintf(os.Stderr, "wrote %s: signal=%s max sustainable %s/sec = %.0f\n",
			cfg.reportOut, cfg.signal, cfg.signal, out.MaxSustainableItemsPerSec)
	case "read-probe":
		fmt.Fprintf(os.Stderr, "wrote %s: signal=%s max fill level passed = %d rows\n",
			cfg.reportOut, cfg.signal, out.MaxFillLevelPassed)
	}

	if chRestarted.Load() {
		fmt.Fprintln(os.Stderr, "FAILED: ClickHouse restarted during the run; treating result as invalid")
		os.Exit(1)
	}
	// Exit 0 unlike chRestarted: passing steps are still a valid lower bound.
	if sutDied.Load() {
		fmt.Fprintln(os.Stderr, "WARNING: SUT died after a failed step and never recovered; headline is a lower bound (cliff not bisected)")
	}
}

// waitForSutHealthy polls GET <target>/health until it returns 200 or the
// timeout expires. Used between phases to detect SUT-side crashes: when
// Phase 1's heavy final step OOM-kills ClickHouse, the compose restart
// policy (on-failure:3) brings the container back within a few seconds.
// This poll confirms recovery before we ask Phase 2 to fire requests at a
// SUT that may still be coming up.
//
// /health returns 200 unconditionally when traceway is alive — it doesn't
// validate downstream CH connectivity. That's intentional: the per-step
// ingestErrThreshold + softCliffRatio already detect CH-down-but-traceway-up,
// so this poll only catches the case where the traceway container itself
// went away.
func waitForSutHealthy(ctx context.Context, target string, timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 5 * time.Second}
	interval := 2 * time.Second
	var lastErr error
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		resp, err := client.Get(target + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("/health did not return 200 within %v (last err: %v)", timeout, lastErr)
		}
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// preWaitCooldown honours the deprecated --inter-phase-cooldown-seconds flag
// when set. Most invocations leave it at 0 and rely entirely on
// waitForMergesIdle; this hook stays so existing local scripts that still pass
// the old flag don't silently skip the wait.
func preWaitCooldown(ctx context.Context, cfg config) {
	if cfg.interPhaseCooldownSeconds <= 0 {
		return
	}
	fmt.Fprintf(stderrPrefix(), "inter-phase cooldown (deprecated flag): %v\n", cfg.interPhaseCooldownSeconds)
	select {
	case <-time.After(cfg.interPhaseCooldownSeconds):
	case <-ctx.Done():
	}
}

// waitForMergesIdle polls /health/deep every 5s until activeMerges == 0 AND
// partsCount has been stable (within ±5%) for two consecutive polls, OR the
// configured timeout elapses. Returns without error in all cases — the wait is
// best-effort observability, not a verdict; if CH never settles we just move
// on and the next phase's results will show it.
func waitForMergesIdle(ctx context.Context, cfg config, client *http.Client, label string) {
	if cfg.maxMergeIdleWait <= 0 {
		return
	}

	// SQLite mode has no ClickHouse — /health/deep returns chReachable:false
	// and there are no merges to wait for. Without this fast-path the loop
	// would burn the full --max-merge-idle-wait (default 5m) between every
	// phase doing nothing useful.
	if first, _ := fetchDeepHealth(ctx, cfg, client); !first.Reachable {
		fmt.Fprintf(stderrPrefix(), "merge-idle [%s]: CH not reachable (sqlite mode or backend down) — skipping wait\n", label)
		return
	}

	deadline := time.Now().Add(cfg.maxMergeIdleWait)
	pollEvery := 5 * time.Second
	stableThreshold := 0.05
	requiredStable := 2

	var prevParts int64 = -1
	stable := 0
	for {
		if ctx.Err() != nil {
			return
		}
		snap, _ := fetchDeepHealth(ctx, cfg, client)
		fmt.Fprintf(stderrPrefix(), "merge-idle [%s]: reachable=%t activeMerges=%d partsCount=%d longestMerge=%.1fs\n",
			label, snap.Reachable, snap.ActiveMerges, snap.PartsCount, snap.LongestMergeSec)

		if snap.Reachable && snap.ActiveMerges == 0 {
			if prevParts >= 0 {
				delta := float64(snap.PartsCount-prevParts) / float64(maxInt64(prevParts, 1))
				if delta < 0 {
					delta = -delta
				}
				if delta <= stableThreshold {
					stable++
					if stable >= requiredStable {
						fmt.Fprintf(stderrPrefix(), "merge-idle [%s]: settled (parts=%d)\n", label, snap.PartsCount)
						return
					}
				} else {
					stable = 0
				}
			}
			prevParts = snap.PartsCount
		} else {
			stable = 0
			if snap.Reachable {
				prevParts = snap.PartsCount
			}
		}

		if time.Now().After(deadline) {
			fmt.Fprintf(stderrPrefix(), "merge-idle [%s]: timeout after %v (activeMerges=%d partsCount=%d) — advancing anyway\n",
				label, cfg.maxMergeIdleWait, snap.ActiveMerges, snap.PartsCount)
			return
		}
		select {
		case <-time.After(pollEvery):
		case <-ctx.Done():
			return
		}
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// writeReportAtomic encodes the report into a sibling .tmp file and renames
// it over the destination, so a kill or disk-full mid-write can't leave a
// half-encoded file. Called after every step.
func writeReportAtomic(path string, report *finalReport) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func parseInt64s(s string) ([]int64, error) {
	parts := strings.Split(s, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", p, err)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty list")
	}
	return out, nil
}

func parseInts(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", p, err)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty list")
	}
	return out, nil
}

func parseFloats(s string) ([]float64, error) {
	parts := strings.Split(s, ",")
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", p, err)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty list")
	}
	return out, nil
}
