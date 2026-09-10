package agent

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

const (
	// LeaseDuration is how long an executor owns a claimed attempt without
	// renewing; every event flush renews it, so a dead executor is noticed
	// within a couple of minutes.
	LeaseDuration     = 2 * time.Minute
	reclaimInterval   = time.Minute
	queueAdvisoryLock = 824737006
)

var wakeCh = make(chan struct{}, 1)

// Wake nudges executors so a freshly queued attempt starts within a second
// instead of at the next poll. Non-blocking.
func Wake() {
	select {
	case wakeCh <- struct{}{}:
	default:
	}
}

// WakeChannel is what in-process executors select on next to their ticker.
func WakeChannel() <-chan struct{} {
	return wakeCh
}

// Claim takes up to limit queued attempts for an executor under a fresh
// lease, in one transaction serialized across instances on Postgres. Each
// claim is status-guarded, so two claimers never take the same attempt.
func Claim(tx *sql.Tx, executor string, limit int, now time.Time) ([]*models.AgentAttempt, error) {
	if !db.IsSQLite() {
		if _, err := tx.Exec(fmt.Sprintf("SELECT pg_advisory_xact_lock(%d)", queueAdvisoryLock)); err != nil {
			return nil, fmt.Errorf("failed to take agent queue lock: %w", err)
		}
	}
	candidates, err := transactional.AgentAttemptRepository.FindClaimable(tx, limit)
	if err != nil {
		return nil, err
	}
	claimed := make([]*models.AgentAttempt, 0, len(candidates))
	for _, attempt := range candidates {
		won, err := transactional.AgentAttemptRepository.Claim(tx, attempt.Id, executor, now.Add(LeaseDuration), now)
		if err != nil {
			return nil, err
		}
		if !won {
			continue
		}
		if _, err := AppendEvent(tx, attempt.Id, EventStatus, map[string]any{"status": models.AttemptClaimed, "executor": executor}, now); err != nil {
			return nil, err
		}
		attempt, err = transactional.AgentAttemptRepository.FindById(tx, attempt.Id)
		if err != nil {
			return nil, err
		}
		claimed = append(claimed, attempt)
	}
	return claimed, nil
}

// Renew extends the lease of an attempt the executor still holds. False
// means the lease was reclaimed and handed elsewhere; the executor must
// stop working on it.
func Renew(tx *sql.Tx, id uuid.UUID, executor string, now time.Time) (bool, error) {
	return transactional.AgentAttemptRepository.RenewLease(tx, id, executor, now.Add(LeaseDuration), now)
}

// ReclaimStale returns attempts whose lease expired to the queue, flagged to
// resume, and records it on each of them.
func ReclaimStale(tx *sql.Tx, now time.Time) (int64, error) {
	stale, err := transactional.AgentAttemptRepository.FindLeaseExpired(tx, now)
	if err != nil {
		return 0, err
	}
	reclaimed, err := transactional.AgentAttemptRepository.ReclaimStale(tx, now)
	if err != nil {
		return 0, err
	}
	for _, attempt := range stale {
		if _, err := AppendEvent(tx, attempt.Id, EventReclaimed, map[string]any{"previousExecutor": attempt.ClaimedBy, "previousStatus": attempt.Status}, now); err != nil {
			return reclaimed, err
		}
	}
	return reclaimed, nil
}

// StartReclaimer returns expired leases to the queue once a minute so a
// crashed executor's attempt is picked up again.
func StartReclaimer(ctx context.Context) {
	go func() {
		defer traceway.Recover()
		ticker := time.NewTicker(reclaimInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			reclaimed, err := db.ExecuteTransaction(func(tx *sql.Tx) (int64, error) {
				return ReclaimStale(tx, time.Now().UTC())
			})
			if err != nil {
				traceway.CaptureException(fmt.Errorf("agent: reclaim stale attempts: %w", err))
				continue
			}
			if reclaimed > 0 {
				recordReclaim(reclaimed, time.Now().UTC())
				config.Logf("agent: returned %d attempt(s) with an expired lease to the queue", reclaimed)
				Wake()
			}
		}
	}()
}
