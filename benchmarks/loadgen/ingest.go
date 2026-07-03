package main

import (
	"context"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

type ingester struct {
	cfg     config
	client  *http.Client
	stats   *latencyTracker
	sender  signalSender
	limiter *rate.Limiter

	batchSize      atomic.Int64
	requestRate    atomic.Int64 // requests/sec, scaled by 1e3 so we keep fractional resolution
	attemptedItems atomic.Int64
	rejectedItems  atomic.Int64

	// rng pool: each worker grabs one to avoid contending on the global source
	rngPool sync.Pool

	// Two contexts let the ramp stop accepting new requests at step boundaries
	// without killing in-flight HTTP. acceptCancel ends the limiter.Wait loop;
	// the request ctx (parent of acceptCtx) remains alive so HTTP responses
	// can complete and be counted. Stop() forces a hard cancel by deriving the
	// hard-stop ctx from the parent.
	workerWg     *sync.WaitGroup
	acceptCancel context.CancelFunc
}

func newIngester(cfg config, client *http.Client, stats *latencyTracker) (*ingester, error) {
	sender, err := pickSender(cfg.signal)
	if err != nil {
		return nil, err
	}
	return &ingester{
		cfg:     cfg,
		client:  client,
		stats:   stats,
		sender:  sender,
		limiter: rate.NewLimiter(rate.Limit(1), 1),
		rngPool: sync.Pool{New: func() any { return rand.New(rand.NewSource(rand.Int63())) }},
	}, nil
}

func pickSender(signal string) (signalSender, error) {
	switch signal {
	case "spans":
		return spansSender{}, nil
	case "metrics":
		return metricsSender{}, nil
	case "logs":
		return logsSender{}, nil
	default:
		return nil, errUnknownSignal(signal)
	}
}

type errUnknownSignal string

func (e errUnknownSignal) Error() string {
	return "unknown signal: " + string(e) + " (expected spans|metrics|logs)"
}

func (i *ingester) Name() string {
	return i.sender.Name()
}

func (i *ingester) SetBatchSize(b int) {
	if b < 1 {
		b = 1
	}
	i.batchSize.Store(int64(b))
}

func (i *ingester) SetRequestRate(rps float64) {
	if rps < 0.001 {
		rps = 0.001
	}
	i.limiter.SetLimit(rate.Limit(rps))
	i.limiter.SetBurst(burstFor(rps))
	i.requestRate.Store(int64(rps * 1000))
}

func burstFor(rps float64) int {
	burst := int(rps) + 1
	if burst < 1 {
		burst = 1
	}
	return burst
}

func (i *ingester) RequestRate() float64 {
	return float64(i.requestRate.Load()) / 1000.0
}

// SnapshotAndResetItems returns attempted/rejected since the last call and
// resets them. Called at step boundaries alongside latencyTracker.
func (i *ingester) SnapshotAndResetItems() (attempted, rejected int64) {
	attempted = i.attemptedItems.Swap(0)
	rejected = i.rejectedItems.Swap(0)
	return
}

// Start launches the worker pool sized for the current request rate. The
// passed ctx is used directly for HTTP requests so in-flight requests survive
// across step boundaries; the limiter loop uses a derived acceptCtx that
// StopAccepting can cancel independently.
func (i *ingester) Start(ctx context.Context) {
	// Rebuild the limiter so each step starts debt-free. Stopping a step
	// mass-cancels every worker blocked in limiter.Wait, and canceled
	// reservations do not restore tokens already consumed by later queued
	// reservations — at low rates that leaves up to workers/rate seconds
	// (e.g. 150 workers at 5 req/s = 30s) of debt on the shared limiter,
	// silently eating the head of the next step. Safe to reassign here:
	// the previous step's Stop() has already joined all workers.
	rps := i.RequestRate()
	if rps <= 0 {
		rps = 1 // Start before any SetRequestRate — match the constructor's default
	}
	i.limiter = rate.NewLimiter(rate.Limit(rps), burstFor(rps))

	acceptCtx, cancelAccept := context.WithCancel(ctx)
	wg := &sync.WaitGroup{}
	i.acceptCancel = cancelAccept
	i.workerWg = wg

	workers := workerCountFor(i.RequestRate())
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			rng := i.rngPool.Get().(*rand.Rand)
			defer i.rngPool.Put(rng)
			for {
				if err := i.limiter.Wait(acceptCtx); err != nil {
					return
				}
				batchSize := int(i.batchSize.Load())
				// HTTP uses the parent ctx (not acceptCtx) so requests in
				// flight when StopAccepting fires aren't canceled mid-flight.
				sendOneOTLP(ctx, i.client, i.cfg, i.sender, rng, batchSize, i.stats, &i.attemptedItems, &i.rejectedItems)
			}
		}()
	}
}

// StopAccepting tells the worker pool to stop pulling from the rate limiter.
// In-flight HTTP requests continue under the parent ctx until they finish or
// the parent ctx is canceled. Pair with WaitForDrain to give them a grace
// window before the next step starts.
func (i *ingester) StopAccepting() {
	if i.acceptCancel != nil {
		i.acceptCancel()
	}
}

// WaitForDrain blocks up to d for the worker pool to finish any in-flight
// HTTP requests. Returns earlier when all workers exit.
func (i *ingester) WaitForDrain(d time.Duration) {
	if i.workerWg == nil {
		return
	}
	done := make(chan struct{})
	go func() { i.workerWg.Wait(); close(done) }()
	if d <= 0 {
		<-done
		return
	}
	select {
	case <-done:
	case <-time.After(d):
	}
}

// Stop ends acceptance and waits forever for the pool to drain. The parent
// ctx still gates HTTP cancellation, so this won't hang past the overall run
// deadline.
func (i *ingester) Stop() {
	i.StopAccepting()
	i.WaitForDrain(0)
	i.acceptCancel = nil
	i.workerWg = nil
}

// workerCountFor scales worker count with request rate. The headroom is
// generous because high-latency steps (multi-second P50) can otherwise starve
// the rate limiter: at rps=5 with P50=4s, 10 workers only deliver 2.5 req/sec.
// 30 seconds of in-flight headroom guarantees we hit target rate unless the
// SUT is truly past its cliff.
func workerCountFor(rps float64) int {
	n := int(rps * 30)
	if n < 32 {
		n = 32
	}
	if n > 512 {
		n = 512
	}
	return n
}
