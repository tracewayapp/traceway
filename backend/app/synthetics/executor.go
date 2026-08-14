package synthetics

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

// browserExecWakeCh nudges the embedded browser executor loop separately from
// the http/tcp loop, so a 2-minute browser run never delays fast probes.
var browserExecWakeCh = make(chan struct{}, 1)

func startExecutor(ctx context.Context) {
	go executorLoop(ctx, wakeCh, func(now time.Time) {
		executeBatch(ctx, now, [3]string{models.CheckTypeHttp, models.CheckTypeTcp, models.CheckTypeTcp}, httpConcurrency())
	})
	if BrowserMode() == BrowserModeEmbedded {
		go executorLoop(ctx, browserExecWakeCh, func(now time.Time) {
			executeBatch(ctx, now, [3]string{models.CheckTypeBrowser, models.CheckTypeBrowser, models.CheckTypeBrowser}, browserConcurrency())
		})
	}
}

func executorLoop(ctx context.Context, wake <-chan struct{}, tick func(now time.Time)) {
	defer traceway.Recover()

	ticker := time.NewTicker(pollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-wake:
		}
		tick(time.Now().UTC())
	}
}

// ExecuteOnce claims and executes every currently claimable run class this
// instance handles. Exported so tests can drive the executor deterministically.
func ExecuteOnce(ctx context.Context, now time.Time) {
	executeBatch(ctx, now, [3]string{models.CheckTypeHttp, models.CheckTypeTcp, models.CheckTypeTcp}, httpConcurrency())
	if BrowserMode() == BrowserModeEmbedded {
		executeBatch(ctx, now, [3]string{models.CheckTypeBrowser, models.CheckTypeBrowser, models.CheckTypeBrowser}, browserConcurrency())
	}
}

func executeBatch(ctx context.Context, now time.Time, types [3]string, concurrency int) {
	type claimedRun struct {
		run   *models.CheckRun
		check *models.SyntheticCheck
	}
	claimedCount := 0
	claimed, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]claimedRun, error) {
		due, err := transactional.CheckRunRepository.FindClaimable(tx, types[0], types[1], types[2], now, claimBatchSize)
		if err != nil {
			return nil, err
		}
		claimedCount = len(due)
		claimed := make([]claimedRun, 0, len(due))
		for _, run := range due {
			won, err := transactional.CheckRunRepository.Claim(tx, run.Id, instanceName, now)
			if err != nil {
				return nil, err
			}
			if !won {
				continue
			}
			check, err := transactional.SyntheticCheckRepository.FindById(tx, run.CheckId)
			if err != nil {
				return nil, err
			}
			if check == nil {
				// Deleted since the run was queued; the cascade removed the
				// row already (or will), nothing to execute.
				continue
			}
			run.ClaimedBy = instanceName
			claimed = append(claimed, claimedRun{run: run, check: check})
		}
		return claimed, nil
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("synthetics executor claim failed: %w", err))
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, item := range claimed {
		wg.Add(1)
		sem <- struct{}{}
		go func(item claimedRun) {
			defer traceway.Recover()
			defer wg.Done()
			defer func() { <-sem }()
			outcome := executeProbe(ctx, item.check)
			ProcessOutcome(ctx, item.run, instanceName, outcome)
		}(item)
	}
	wg.Wait()

	if claimedCount == claimBatchSize {
		// A full batch means more runs may already be claimable.
		Wake()
	}
}

// executeProbe runs one check with its timeout and normalizes the outcome.
func executeProbe(ctx context.Context, check *models.SyntheticCheck) Outcome {
	probeCtx, cancel := context.WithTimeout(ctx, EffectiveTimeout(check))
	defer cancel()

	started := time.Now().UTC()
	var outcome Outcome
	switch check.CheckType {
	case models.CheckTypeHttp:
		outcome = probeHttp(probeCtx, check)
	case models.CheckTypeTcp:
		outcome = probeTcp(probeCtx, check)
	case models.CheckTypeBrowser:
		outcome = probeBrowser(probeCtx, check)
	default:
		outcome = Outcome{Status: models.CheckResultDown, ErrorMsg: "unknown check type " + check.CheckType, TlsDaysRemaining: -1}
	}
	outcome.ExecutedAt = started
	if outcome.ExecutedBy == "" {
		outcome.ExecutedBy = "embedded"
	}
	return outcome
}
