//go:build transactional_pg

package pg

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type agentProfileRepository struct{}

const agentProfileColumns = "id, organization_id, name, agent, package, package_version, model, provider, base_url, credential, max_turns, timeout_minutes, budget_usd, allowed_tools, network_policy, is_default, created_at, updated_at"

func (r *agentProfileRepository) Create(tx *sql.Tx, profile *models.AgentProfile) (int, error) {
	return lit.Insert[models.AgentProfile](tx, profile)
}

func (r *agentProfileRepository) FindById(tx *sql.Tx, id int) (*models.AgentProfile, error) {
	return lit.SelectSingleNamed[models.AgentProfile](
		tx,
		"SELECT "+agentProfileColumns+" FROM agent_profiles WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *agentProfileRepository) FindByOrganization(tx *sql.Tx, organizationId int) ([]*models.AgentProfile, error) {
	return lit.SelectNamed[models.AgentProfile](
		tx,
		"SELECT "+agentProfileColumns+" FROM agent_profiles WHERE organization_id = :organization_id ORDER BY is_default DESC, name ASC",
		lit.P{"organization_id": organizationId},
	)
}

// FindDefault returns the organization's default profile, or nil.
func (r *agentProfileRepository) FindDefault(tx *sql.Tx, organizationId int) (*models.AgentProfile, error) {
	return lit.SelectSingleNamed[models.AgentProfile](
		tx,
		"SELECT "+agentProfileColumns+" FROM agent_profiles WHERE organization_id = :organization_id AND is_default = true ORDER BY id ASC LIMIT 1",
		lit.P{"organization_id": organizationId},
	)
}

func (r *agentProfileRepository) Update(tx *sql.Tx, profile *models.AgentProfile) error {
	return lit.UpdateNamed(tx, profile, "id = :id", lit.P{"id": profile.Id})
}

// SetDefault makes one profile the organization's default and clears the
// flag on every other one, so there is never more than one.
func (r *agentProfileRepository) SetDefault(tx *sql.Tx, organizationId int, id int, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE agent_profiles SET is_default = (id = :id), updated_at = :now WHERE organization_id = :organization_id",
		lit.P{"id": id, "now": now.UTC(), "organization_id": organizationId},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *agentProfileRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM agent_profiles WHERE id = :id", lit.P{"id": id})
}

var AgentProfileRepository = agentProfileRepository{}
