//go:build transactional_pg

package pg

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type agentLinkRepository struct{}

const agentLinkColumns = "id, attempt_id, integration_id, provider, kind, external_ref, url, created_at"

// Create records an external artifact. The unique (provider, kind,
// external_ref) index rejects a second attempt claiming the same artifact.
func (r *agentLinkRepository) Create(tx *sql.Tx, link *models.AgentLink) (int, error) {
	return lit.Insert[models.AgentLink](tx, link)
}

func (r *agentLinkRepository) FindByAttempt(tx *sql.Tx, attemptId uuid.UUID) ([]*models.AgentLink, error) {
	return lit.SelectNamed[models.AgentLink](
		tx,
		"SELECT "+agentLinkColumns+" FROM agent_links WHERE attempt_id = :attempt_id ORDER BY id ASC",
		lit.P{"attempt_id": attemptId},
	)
}

func (r *agentLinkRepository) FindByAttemptAndKind(tx *sql.Tx, attemptId uuid.UUID, provider string, kind string) (*models.AgentLink, error) {
	return lit.SelectSingleNamed[models.AgentLink](
		tx,
		"SELECT "+agentLinkColumns+" FROM agent_links WHERE attempt_id = :attempt_id AND provider = :provider AND kind = :kind ORDER BY id ASC LIMIT 1",
		lit.P{"attempt_id": attemptId, "provider": provider, "kind": kind},
	)
}

// FindByExternalRef resolves an inbound provider event (a reply in a thread,
// a merged pull request) to the attempt that owns the artifact.
func (r *agentLinkRepository) FindByExternalRef(tx *sql.Tx, provider string, kind string, externalRef string) (*models.AgentLink, error) {
	return lit.SelectSingleNamed[models.AgentLink](
		tx,
		"SELECT "+agentLinkColumns+" FROM agent_links WHERE provider = :provider AND kind = :kind AND external_ref = :external_ref",
		lit.P{"provider": provider, "kind": kind, "external_ref": externalRef},
	)
}

func (r *agentLinkRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM agent_links WHERE id = :id", lit.P{"id": id})
}

func (r *agentLinkRepository) FindInbound(tx *sql.Tx, integrationId int, provider, kind, ref string) (*models.AgentLink, error) {
	return lit.SelectSingleNamed[models.AgentLink](tx,
		"SELECT "+agentLinkColumns+" FROM agent_links WHERE integration_id = :integration_id AND provider = :provider AND kind = :kind AND external_ref = :ref AND attempt_id IN (SELECT id FROM agent_attempts WHERE status IN "+statusList(models.AttemptActiveStatuses)+") ORDER BY id DESC LIMIT 1",
		lit.P{"integration_id": integrationId, "provider": provider, "kind": kind, "ref": ref})
}

func (r *agentLinkRepository) FindForAttempt(tx *sql.Tx, attemptId uuid.UUID, integrationId int, provider, kind, ref string) (*models.AgentLink, error) {
	return lit.SelectSingleNamed[models.AgentLink](tx,
		"SELECT "+agentLinkColumns+" FROM agent_links WHERE attempt_id = :attempt_id AND COALESCE(integration_id, 0) = :integration_id AND provider = :provider AND kind = :kind AND external_ref = :ref",
		lit.P{"attempt_id": attemptId, "integration_id": integrationId, "provider": provider, "kind": kind, "ref": ref})
}

var AgentLinkRepository = agentLinkRepository{}
