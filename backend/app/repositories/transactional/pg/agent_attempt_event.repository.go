//go:build transactional_pg

package pg

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type agentAttemptEventRepository struct{}

const agentAttemptEventColumns = "id, attempt_id, seq, kind, payload, created_at"

// Append adds the next event of an attempt. The sequence is read and written
// in the caller's transaction; the unique (attempt_id, seq) index turns a
// concurrent append into an error rather than a gap or a duplicate.
func (r *agentAttemptEventRepository) Append(tx *sql.Tx, attemptId uuid.UUID, kind string, payload models.JSONText, now time.Time) (*models.AgentAttemptEvent, error) {
	seq, err := r.LatestSeq(tx, attemptId)
	if err != nil {
		return nil, err
	}
	event := &models.AgentAttemptEvent{
		AttemptId: attemptId,
		Seq:       seq + 1,
		Kind:      kind,
		Payload:   payload,
		CreatedAt: now.UTC(),
	}
	id, err := lit.Insert[models.AgentAttemptEvent](tx, event)
	if err != nil {
		return nil, err
	}
	event.Id = id
	return event, nil
}

func (r *agentAttemptEventRepository) LatestSeq(tx *sql.Tx, attemptId uuid.UUID) (int, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COALESCE(MAX(seq), 0) AS count FROM agent_attempt_events WHERE attempt_id = :attempt_id",
		lit.P{"attempt_id": attemptId},
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.Count, nil
}

// ListAfter returns the events with a sequence above afterSeq, in order:
// the poll cursor the run page and the executors use.
func (r *agentAttemptEventRepository) ListAfter(tx *sql.Tx, attemptId uuid.UUID, afterSeq int, limit int) ([]*models.AgentAttemptEvent, error) {
	return lit.SelectNamed[models.AgentAttemptEvent](
		tx,
		"SELECT "+agentAttemptEventColumns+" FROM agent_attempt_events WHERE attempt_id = :attempt_id AND seq > :after_seq ORDER BY seq ASC LIMIT :limit",
		lit.P{"attempt_id": attemptId, "after_seq": afterSeq, "limit": limit},
	)
}

func (r *agentAttemptEventRepository) DeleteByAttempt(tx *sql.Tx, attemptId uuid.UUID) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM agent_attempt_events WHERE attempt_id = :attempt_id",
		lit.P{"attempt_id": attemptId},
	)
	if err != nil {
		return 0, err
	}
	result, err := tx.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

var AgentAttemptEventRepository = agentAttemptEventRepository{}
