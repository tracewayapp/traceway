package synthetics

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/synthetics/browserexec"
	traceway "go.tracewayapp.com"
)

var missedTotal uint64

// Start runs the scheduler and the in-process executor.
func Start(ctx context.Context) {
	// Browser checks are arbitrary code execution on the server; they must be
	// impossible to enable in cloud mode.
	if config.Config.CloudMode == "true" && BrowserMode() != BrowserModeOff {
		panic("SYNTHETICS_BROWSER_MODE must be off in cloud mode: browser checks execute user-authored scripts on the server")
	}
	// Fail fast instead of failing every browser run: embedded mode is only
	// meaningful with the Node harness present (the :browser image).
	if BrowserMode() == BrowserModeEmbedded {
		if err := browserexec.Validate(PlaywrightDir()); err != nil {
			panic(fmt.Sprintf("SYNTHETICS_BROWSER_MODE=embedded but the Playwright harness is unusable (%v); use the ghcr.io/tracewayapp/traceway:browser image or point SYNTHETICS_PLAYWRIGHT_DIR at a directory with @playwright/test installed", err))
		}
	}
	// Same fail-fast principle for remote mode: without the shared secret no
	// runner can ever authenticate, so every browser run would expire missed.
	if BrowserMode() == BrowserModeRemote && config.Config.SyntheticsRunnerSecret == "" {
		panic("SYNTHETICS_BROWSER_MODE=remote requires SYNTHETICS_RUNNER_SECRET so runners can authenticate")
	}
	config.Logf("Starting synthetics scheduler (interval: %s, browser mode: %s)", pollInterval(), BrowserMode())

	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(pollInterval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			ScheduleOnce(ctx, time.Now().UTC())
		}
	}()

	startExecutor(ctx)
}

// ScheduleOnce runs a single scheduler tick: reclaim stale claims, expire
// overdue queued runs as missed, enqueue due checks, advance their
// next_run_at. Exported so tests can drive the scheduler deterministically.
func ScheduleOnce(ctx context.Context, now time.Time) {
	type tickResult struct {
		missed       []*models.CheckRun
		enqueued     int
		browserRuns  bool
		dueSaturated bool
	}
	result, err := db.ExecuteTransaction(func(tx *sql.Tx) (tickResult, error) {
		var res tickResult
		if !db.IsSQLite() {
			if _, err := tx.Exec(fmt.Sprintf("SELECT pg_advisory_xact_lock(%d)", schedulerAdvisoryLockId)); err != nil {
				return res, fmt.Errorf("failed to take synthetics scheduler lock: %w", err)
			}
		}
		if err := transactional.CheckRunRepository.ReclaimStaleClaimed(tx, now.Add(-staleClaimAge)); err != nil {
			return res, err
		}

		expired, err := transactional.CheckRunRepository.FindExpiredQueued(tx, now, expiredBatchSize)
		if err != nil {
			return res, err
		}
		for _, run := range expired {
			if err := transactional.CheckRunRepository.Delete(tx, run.Id); err != nil {
				return res, err
			}
		}
		res.missed = expired

		due, err := transactional.SyntheticCheckRepository.FindDueEnabled(tx, now, scheduleBatchSize)
		if err != nil {
			return res, err
		}
		res.dueSaturated = len(due) == scheduleBatchSize
		for _, check := range due {
			run := &models.CheckRun{
				CheckId:      check.Id,
				ProjectId:    check.ProjectId,
				CheckType:    check.CheckType,
				Status:       models.CheckRunQueued,
				ScheduledFor: now,
				ExpiresAt:    RunExpiry(now, check.IntervalSeconds),
				CreatedAt:    now,
			}
			if _, err := transactional.CheckRunRepository.Enqueue(tx, run); err != nil {
				return res, err
			}
			if err := transactional.SyntheticCheckRepository.SetNextRun(tx, check.Id, now.Add(time.Duration(check.IntervalSeconds)*time.Second)); err != nil {
				return res, err
			}
			res.enqueued++
			if check.CheckType == models.CheckTypeBrowser {
				res.browserRuns = true
			}
		}
		return res, nil
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("synthetics scheduler tick failed: %w", err))
		return
	}

	// Missed runs are recorded to telemetry after the delete committed: a
	// crash in between loses the missed marker (a gap), never duplicates it.
	for _, run := range result.missed {
		atomic.AddUint64(&missedTotal, 1)
		insertErr := telemetry.CheckResultRepository.Insert(ctx, telemetry.CheckResult{
			ProjectId:        run.ProjectId,
			CheckId:          run.CheckId,
			RunId:            run.Id,
			CheckType:        run.CheckType,
			RecordedAt:       run.ScheduledFor,
			ExecutedAt:       now,
			Status:           models.CheckResultMissed,
			ExecutedBy:       "scheduler",
			TlsDaysRemaining: -1,
		})
		if insertErr != nil {
			traceway.CaptureException(fmt.Errorf("failed to record missed check result (check=%d run=%d): %w", run.CheckId, run.Id, insertErr))
		}
	}

	if result.enqueued > 0 || result.dueSaturated {
		Wake()
	}
	if result.browserRuns {
		wakeBrowser()
	}
}
