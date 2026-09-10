package agent

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// allowedFrom is the attempt state machine: for each target status, the
// statuses it may be reached from. Terminal statuses have no exits.
var allowedFrom = map[string][]string{
	models.AttemptQueued:         {models.AttemptPendingApproval, models.AttemptNeedsInput, models.AttemptAwaitingReview},
	models.AttemptClaimed:        {models.AttemptQueued},
	models.AttemptPreparing:      {models.AttemptClaimed},
	models.AttemptRunning:        {models.AttemptPreparing, models.AttemptVerifying},
	models.AttemptVerifying:      {models.AttemptRunning},
	models.AttemptPublishing:     {models.AttemptVerifying},
	models.AttemptNeedsInput:     {models.AttemptRunning},
	models.AttemptAwaitingReview: {models.AttemptPublishing},
	models.AttemptAnalyzed:       {models.AttemptPreparing, models.AttemptRunning, models.AttemptVerifying},
	models.AttemptMerged:         {models.AttemptAwaitingReview},
	models.AttemptClosed:         {models.AttemptAwaitingReview},
	models.AttemptFailed:         {models.AttemptClaimed, models.AttemptPreparing, models.AttemptRunning, models.AttemptVerifying, models.AttemptPublishing},
	models.AttemptTimedOut:       {models.AttemptClaimed, models.AttemptPreparing, models.AttemptRunning, models.AttemptVerifying, models.AttemptPublishing, models.AttemptNeedsInput, models.AttemptAwaitingReview},
	models.AttemptCancelled:      models.AttemptActiveStatuses,
}

var ErrIllegalTransition = errors.New("illegal attempt transition")

// CanTransition reports whether the state machine allows from -> to.
func CanTransition(from, to string) bool {
	for _, allowed := range allowedFrom[to] {
		if allowed == from {
			return true
		}
	}
	return false
}

// Transition moves an attempt to a status when the machine allows it from
// the status the row is in right now, and records the change as an event.
// A lost race or an illegal move returns ErrIllegalTransition.
func Transition(tx *sql.Tx, id uuid.UUID, to string, now time.Time) error {
	from, ok := allowedFrom[to]
	if !ok {
		return fmt.Errorf("%w: unknown status %q", ErrIllegalTransition, to)
	}
	moved, err := transactional.AgentAttemptRepository.Transition(tx, id, from, to, now)
	if err != nil {
		return err
	}
	if !moved {
		return fmt.Errorf("%w: attempt %s cannot move to %s", ErrIllegalTransition, id, to)
	}
	if slices.Contains(models.AttemptTerminalStatuses, to) {
		recordFinished(to)
	}
	_, err = AppendEvent(tx, id, EventStatus, map[string]any{"status": to}, now)
	return err
}
