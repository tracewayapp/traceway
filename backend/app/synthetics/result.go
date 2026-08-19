package synthetics

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

var executedTotal uint64

// Outcome is one executed probe's result, produced by the in-process runners
// or posted by a remote runner.
type Outcome struct {
	Status           string // models.CheckResultUp or models.CheckResultDown
	LatencyMs        float64
	StatusCode       int
	ErrorMsg         string
	ScreenshotKey    string
	OutputKey        string
	TlsDaysRemaining int
	ExecutedBy       string
	ExecutedAt       time.Time
}

// ProcessOutcome finalizes one executed run. Order matters: the main-DB
// transaction (state transition + incident + run-row delete) commits FIRST,
// then the telemetry insert, then the notify hook. A crash between the two
// DB writes loses one result row (an uptime gap), which is benign; the
// reverse order would re-execute after reclaim and double-apply the state
// transition. The notify hook runs with no transaction held: it opens its
// own, and the SQLite main DB has a single connection.
func ProcessOutcome(ctx context.Context, run *models.CheckRun, claimedBy string, outcome Outcome) {
	type finalization struct {
		finalized  bool
		check      *models.SyntheticCheck
		fromStatus string
		toStatus   string
	}
	result, err := db.ExecuteTransaction(func(tx *sql.Tx) (finalization, error) {
		var res finalization
		won, err := transactional.CheckRunRepository.DeleteClaimedBy(tx, run.Id, claimedBy)
		if err != nil {
			return res, err
		}
		if !won {
			// A stale-claim reclaim handed this run to someone else, or the
			// check (and its runs, via cascade) was deleted mid-flight.
			return res, nil
		}
		check, err := transactional.SyntheticCheckRepository.FindById(tx, run.CheckId)
		if err != nil {
			return res, err
		}
		if check == nil {
			return res, nil
		}

		fromStatus := check.CurrentStatus
		toStatus := fromStatus
		consecutiveFailures := 0
		if outcome.Status == models.CheckResultUp {
			toStatus = models.CheckStatusUp
		} else {
			consecutiveFailures = check.ConsecutiveFailures + 1
			threshold := check.FailureThreshold
			if threshold < 1 {
				threshold = 1
			}
			if consecutiveFailures >= threshold {
				toStatus = models.CheckStatusDown
			}
		}

		var stateChangedAt *time.Time
		if toStatus != fromStatus {
			stateChangedAt = &outcome.ExecutedAt
		}
		if err := transactional.SyntheticCheckRepository.ApplyRunState(tx, check.Id, toStatus, consecutiveFailures, outcome.ExecutedAt, stateChangedAt); err != nil {
			return res, err
		}
		// Mirror the persisted state onto the struct handed to the notifier;
		// the "N consecutive failures" in the alert must be the post-run count.
		check.CurrentStatus = toStatus
		check.ConsecutiveFailures = consecutiveFailures
		check.LastRunAt = &outcome.ExecutedAt
		if stateChangedAt != nil {
			check.LastStateChangeAt = stateChangedAt
		}

		if toStatus == models.CheckStatusDown && fromStatus != models.CheckStatusDown {
			open, err := transactional.CheckIncidentRepository.FindOpenByCheck(tx, check.Id)
			if err != nil {
				return res, err
			}
			if open == nil {
				incident := &models.CheckIncident{
					CheckId:      &check.Id,
					ProjectId:    &check.ProjectId,
					StartedAt:    outcome.ExecutedAt,
					ErrorMessage: outcome.ErrorMsg,
				}
				incidentId, err := transactional.CheckIncidentRepository.Open(tx, incident)
				if err != nil {
					return res, err
				}
				update := &models.IncidentUpdate{
					IncidentId: incidentId,
					Status:     models.IncidentUpdateInvestigating,
					CreatedAt:  outcome.ExecutedAt,
				}
				if _, err := transactional.IncidentUpdateRepository.Create(tx, update); err != nil {
					return res, err
				}
			}
		}
		if toStatus == models.CheckStatusUp && fromStatus == models.CheckStatusDown {
			open, err := transactional.CheckIncidentRepository.FindOpenByCheck(tx, check.Id)
			if err != nil {
				return res, err
			}
			if open != nil {
				if err := transactional.CheckIncidentRepository.Resolve(tx, open.Id, outcome.ExecutedAt); err != nil {
					return res, err
				}
				update := &models.IncidentUpdate{
					IncidentId: open.Id,
					Status:     models.IncidentUpdateResolved,
					CreatedAt:  outcome.ExecutedAt,
				}
				if _, err := transactional.IncidentUpdateRepository.Create(tx, update); err != nil {
					return res, err
				}
			}
		}

		res.finalized = true
		res.check = check
		res.fromStatus = fromStatus
		res.toStatus = toStatus
		return res, nil
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to finalize check run %d: %w", run.Id, err))
		return
	}
	if !result.finalized {
		return
	}

	atomic.AddUint64(&executedTotal, 1)

	insertErr := telemetry.CheckResultRepository.Insert(ctx, telemetry.CheckResult{
		ProjectId:        run.ProjectId,
		CheckId:          run.CheckId,
		RunId:            run.Id,
		CheckType:        run.CheckType,
		RecordedAt:       run.ScheduledFor,
		ExecutedAt:       outcome.ExecutedAt,
		Status:           outcome.Status,
		LatencyMs:        outcome.LatencyMs,
		StatusCode:       outcome.StatusCode,
		ErrorMsg:         outcome.ErrorMsg,
		ExecutedBy:       outcome.ExecutedBy,
		ScreenshotKey:    outcome.ScreenshotKey,
		OutputKey:        outcome.OutputKey,
		TlsDaysRemaining: outcome.TlsDaysRemaining,
	})
	if insertErr != nil {
		traceway.CaptureException(fmt.Errorf("failed to record check result (check=%d run=%d): %w", run.CheckId, run.Id, insertErr))
	}

	if result.fromStatus == result.toStatus || notifier == nil {
		return
	}
	wentDown := result.toStatus == models.CheckStatusDown
	recovered := result.fromStatus == models.CheckStatusDown && result.toStatus == models.CheckStatusUp
	if !wentDown && !recovered {
		return
	}
	transition := StateTransition{
		Check:    *result.check,
		From:     result.fromStatus,
		To:       result.toStatus,
		ErrorMsg: outcome.ErrorMsg,
		At:       outcome.ExecutedAt,
	}
	go func() {
		defer traceway.Recover()
		notifier(transition)
	}()
}
