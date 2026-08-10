package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

const (
	drainAdvisoryLockId = 824737003 // escalator: 824737002, backfill: 824737001
	drainBatchSize      = 50
	drainConcurrency    = 8
	sendTimeout         = 30 * time.Second
	staleSendingAge     = 5 * time.Minute
	maxAttempts         = 5
)

// backoffSchedule[attempts-1] = delay before the next attempt (attempts was
// already incremented at claim time). attempts >= maxAttempts is terminal.
var backoffSchedule = []time.Duration{1 * time.Minute, 5 * time.Minute, 15 * time.Minute, 60 * time.Minute}

var (
	sentTotal             uint64
	terminalFailuresTotal uint64
)

func drainPollInterval() time.Duration {
	return config.PollSeconds(config.Config.OutboxPollSeconds, 15)
}

// StartDrain runs the outbox drain loop: claim due rows in a transaction
// (marking them sending), deliver after commit, finalize each row in its own
// small transaction. All state lives in notification_outbox, so a crash at any
// point is recovered: unclaimed rows stay due, stale sending rows are
// reclaimed, and the guarded status transitions make cancellation win races.
func StartDrain(ctx context.Context) {
	go func() {
		defer traceway.Recover()

		ticker := time.NewTicker(drainPollInterval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-wakeCh:
			}
			DrainOnce(ctx, time.Now().UTC())
		}
	}()
}

// DrainOnce runs a single drain tick: reclaim stale sending rows, claim due
// rows, deliver them, finalize. Exported so tests (and tooling) can drive the
// outbox deterministically.
func DrainOnce(ctx context.Context, now time.Time) {
	dueCount := 0
	claimed, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.OutboxDelivery, error) {
		if !db.IsSQLite() {
			if _, err := tx.Exec(fmt.Sprintf("SELECT pg_advisory_xact_lock(%d)", drainAdvisoryLockId)); err != nil {
				return nil, fmt.Errorf("failed to take outbox drain lock: %w", err)
			}
		}
		if err := transactional.OutboxRepository.ReclaimStaleSending(tx, now.Add(-staleSendingAge), now); err != nil {
			return nil, err
		}
		due, err := transactional.OutboxRepository.FindDue(tx, now, drainBatchSize)
		if err != nil {
			return nil, err
		}
		dueCount = len(due)
		claimed := make([]*models.OutboxDelivery, 0, len(due))
		for _, row := range due {
			won, err := transactional.OutboxRepository.MarkSending(tx, row.Id, now)
			if err != nil {
				return nil, err
			}
			if !won {
				// Cancelled (acknowledge/resolve) since the due list was read;
				// the row must not be sent.
				continue
			}
			row.Attempts++
			claimed = append(claimed, row)
		}
		return claimed, nil
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("outbox drain tick failed: %w", err))
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, drainConcurrency)
	for _, row := range claimed {
		wg.Add(1)
		sem <- struct{}{}
		go func(row *models.OutboxDelivery) {
			defer traceway.Recover()
			defer wg.Done()
			defer func() { <-sem }()
			sendRow(ctx, row)
		}(row)
	}
	wg.Wait()

	if dueCount == drainBatchSize {
		// A full batch means more rows may already be due.
		Wake()
	}
}

func sendRow(ctx context.Context, row *models.OutboxDelivery) {
	if sender == nil {
		finalizeRow(row, errors.New("outbox sender not registered"))
		return
	}
	var msg models.NotificationMessage
	if err := json.Unmarshal(row.Message, &msg); err != nil {
		// Poison row: retrying cannot fix an unreadable payload.
		finalizeTerminal(row, "unreadable message payload: "+err.Error())
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	finalizeRow(row, sender(sendCtx, row.AdapterType, json.RawMessage(row.AdapterConfig), msg))
}

func finalizeRow(row *models.OutboxDelivery, sendErr error) {
	if sendErr != nil && row.Attempts >= maxAttempts {
		finalizeTerminal(row, sendErr.Error())
		return
	}
	now := time.Now().UTC()
	finalized, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		if sendErr == nil {
			sent, err := transactional.OutboxRepository.MarkSent(tx, row.Id, now)
			if err != nil {
				return false, err
			}
			if !sent {
				// Cancelled while the send was in flight (the documented
				// at-least-once window): the cancel already mirrored the
				// page_notifications row, which must not be rewritten.
				return false, nil
			}
			if row.PageNotificationId != nil {
				if err := transactional.PageNotificationRepository.MarkSent(tx, *row.PageNotificationId, now); err != nil {
					return false, err
				}
			}
			return true, nil
		}
		next := now.Add(backoffSchedule[backoffIndex(row.Attempts)])
		return transactional.OutboxRepository.MarkFailedWithBackoff(tx, row.Id, sendErr.Error(), &next, now)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to finalize outbox row %d: %w", row.Id, err))
		return
	}
	if sendErr != nil {
		// Retryable failures must be visible before they turn terminal.
		traceway.CaptureException(fmt.Errorf("outbox delivery %d (%s via %s) attempt %d failed, will retry: %w", row.Id, row.Kind, row.AdapterType, row.Attempts, sendErr))
		return
	}
	if finalized {
		atomic.AddUint64(&sentTotal, 1)
		if terminalHook != nil {
			terminalHook(row, models.OutboxSent, "")
		}
	}
}

func finalizeTerminal(row *models.OutboxDelivery, errorMsg string) {
	now := time.Now().UTC()
	finalized, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		failed, err := transactional.OutboxRepository.MarkFailedWithBackoff(tx, row.Id, errorMsg, nil, now)
		if err != nil {
			return false, err
		}
		if !failed {
			// Cancelled while the send was in flight; the cancel already
			// mirrored the page_notifications row.
			return false, nil
		}
		if row.PageNotificationId != nil {
			if err := transactional.PageNotificationRepository.MarkFailed(tx, *row.PageNotificationId, errorMsg, now); err != nil {
				return false, err
			}
		}
		return true, nil
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("failed to finalize outbox row %d: %w", row.Id, err))
		return
	}
	if !finalized {
		return
	}
	atomic.AddUint64(&terminalFailuresTotal, 1)
	traceway.CaptureException(fmt.Errorf("outbox delivery %d (%s via %s) permanently failed after %d attempts: %s", row.Id, row.Kind, row.AdapterType, row.Attempts, errorMsg))
	if terminalHook != nil {
		terminalHook(row, models.OutboxFailed, errorMsg)
	}
}

func backoffIndex(attempts int) int {
	index := attempts - 1
	if index < 0 {
		index = 0
	}
	if index >= len(backoffSchedule) {
		index = len(backoffSchedule) - 1
	}
	return index
}
