package agent

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services"
)

// RunTokenTTL is short on purpose: the executor renews while the attempt is
// active, and a leaked token stops working minutes after the run ends.
const RunTokenTTL = 15 * time.Minute

var ErrAttemptNotActive = errors.New("attempt is not active")

// RunToken is the credential the agent process gets: readonly, one project,
// short-lived.
type RunToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// MintRunToken issues a run token for an attempt, or renews one; it refuses
// once the attempt is terminal.
func MintRunToken(tx *sql.Tx, attemptId uuid.UUID) (*RunToken, error) {
	attempt, err := transactional.AgentAttemptRepository.FindById(tx, attemptId)
	if err != nil {
		return nil, err
	}
	if attempt == nil || !models.AttemptIsActive(attempt.Status) {
		return nil, ErrAttemptNotActive
	}
	token, expires, err := services.GenerateRunToken(attempt.Id, attempt.ProjectId, RunTokenTTL)
	if err != nil {
		return nil, err
	}
	return &RunToken{Token: token, ExpiresAt: expires}, nil
}
