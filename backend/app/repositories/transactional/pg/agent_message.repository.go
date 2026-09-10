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

type agentMessageRepository struct{}

const agentMessageColumns = "id, attempt_id, direction, provider, link_id, identity_id, kind, body, external_ref, delivered_at, created_at"

func (r *agentMessageRepository) Create(tx *sql.Tx, message *models.AgentMessage) (int, error) {
	return lit.Insert[models.AgentMessage](tx, message)
}

func (r *agentMessageRepository) FindById(tx *sql.Tx, id int) (*models.AgentMessage, error) {
	return lit.SelectSingleNamed[models.AgentMessage](
		tx,
		"SELECT "+agentMessageColumns+" FROM agent_messages WHERE id = :id",
		lit.P{"id": id},
	)
}

// ListAfter returns the thread after a message id, oldest first; an empty
// direction returns both sides.
func (r *agentMessageRepository) ListAfter(tx *sql.Tx, attemptId uuid.UUID, afterId int, direction string, limit int) ([]*models.AgentMessage, error) {
	return lit.SelectNamed[models.AgentMessage](
		tx,
		"SELECT "+agentMessageColumns+" FROM agent_messages WHERE attempt_id = :attempt_id AND id > :after_id AND (:direction = '' OR direction = :direction) ORDER BY id ASC LIMIT :limit",
		lit.P{"attempt_id": attemptId, "after_id": afterId, "direction": direction, "limit": limit},
	)
}

// FindByExternalRef finds the message a provider event already produced, so
// a redelivered webhook or a mirrored post does not duplicate it.
func (r *agentMessageRepository) FindByExternalRef(tx *sql.Tx, attemptId uuid.UUID, provider string, externalRef string) (*models.AgentMessage, error) {
	return lit.SelectSingleNamed[models.AgentMessage](
		tx,
		"SELECT "+agentMessageColumns+" FROM agent_messages WHERE attempt_id = :attempt_id AND provider = :provider AND external_ref <> '' AND external_ref = :external_ref",
		lit.P{"attempt_id": attemptId, "provider": provider, "external_ref": externalRef},
	)
}

func (r *agentMessageRepository) MarkDelivered(tx *sql.Tx, id int, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE agent_messages SET delivered_at = :now WHERE id = :id",
		lit.P{"now": now.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

var AgentMessageRepository = agentMessageRepository{}
