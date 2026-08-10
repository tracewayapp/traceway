//go:build !transactional_pg

package sqlite

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type escalationPolicyRepository struct{}

const escalationPolicyColumns = "id, organization_id, name, definition, created_by, created_at, updated_at"

func (r *escalationPolicyRepository) FindById(tx *sql.Tx, id int) (*models.EscalationPolicy, error) {
	return lit.SelectSingleNamed[models.EscalationPolicy](
		tx,
		"SELECT "+escalationPolicyColumns+" FROM escalation_policies WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *escalationPolicyRepository) FindByOrganization(tx *sql.Tx, organizationId int) ([]*models.EscalationPolicy, error) {
	return lit.SelectNamed[models.EscalationPolicy](
		tx,
		"SELECT "+escalationPolicyColumns+" FROM escalation_policies WHERE organization_id = :organization_id ORDER BY name ASC, id ASC",
		lit.P{"organization_id": organizationId},
	)
}

func (r *escalationPolicyRepository) FindByOrganizationAndName(tx *sql.Tx, organizationId int, name string) (*models.EscalationPolicy, error) {
	return lit.SelectSingleNamed[models.EscalationPolicy](
		tx,
		"SELECT "+escalationPolicyColumns+" FROM escalation_policies WHERE organization_id = :organization_id AND LOWER(name) = LOWER(:name)",
		lit.P{"organization_id": organizationId, "name": name},
	)
}

func (r *escalationPolicyRepository) Create(tx *sql.Tx, policy *models.EscalationPolicy) (int, error) {
	return lit.Insert[models.EscalationPolicy](tx, policy)
}

func (r *escalationPolicyRepository) Update(tx *sql.Tx, policy *models.EscalationPolicy) error {
	return lit.UpdateNamed(tx, policy, "id = :id", lit.P{"id": policy.Id})
}

func (r *escalationPolicyRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM escalation_policies WHERE id = :id", lit.P{"id": id})
}

var EscalationPolicyRepository = escalationPolicyRepository{}
