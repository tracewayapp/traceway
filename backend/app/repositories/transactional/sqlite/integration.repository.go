//go:build !transactional_pg

package sqlite

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type integrationRepository struct{}

const integrationColumns = "id, organization_id, provider, kinds, name, config, enabled, created_by, created_at, updated_at"

func (r *integrationRepository) Create(tx *sql.Tx, integration *models.Integration) (int, error) {
	return lit.Insert[models.Integration](tx, integration)
}

func (r *integrationRepository) FindById(tx *sql.Tx, id int) (*models.Integration, error) {
	return lit.SelectSingleNamed[models.Integration](
		tx,
		"SELECT "+integrationColumns+" FROM integrations WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *integrationRepository) FindByOrganization(tx *sql.Tx, organizationId int) ([]*models.Integration, error) {
	return lit.SelectNamed[models.Integration](
		tx,
		"SELECT "+integrationColumns+" FROM integrations WHERE organization_id = :organization_id ORDER BY provider ASC, name ASC",
		lit.P{"organization_id": organizationId},
	)
}

// FindEnabledByProvider returns the organization's enabled integrations of
// one provider, the ones a channel or trigger may be routed through.
func (r *integrationRepository) FindEnabledByProvider(tx *sql.Tx, organizationId int, provider string) ([]*models.Integration, error) {
	return lit.SelectNamed[models.Integration](
		tx,
		"SELECT "+integrationColumns+" FROM integrations WHERE organization_id = :organization_id AND provider = :provider AND enabled = true ORDER BY name ASC",
		lit.P{"organization_id": organizationId, "provider": provider},
	)
}

func (r *integrationRepository) FindAllByProvider(tx *sql.Tx, provider string) ([]*models.Integration, error) {
	return lit.SelectNamed[models.Integration](
		tx,
		"SELECT "+integrationColumns+" FROM integrations WHERE provider = :provider ORDER BY id",
		lit.P{"provider": provider},
	)
}

func (r *integrationRepository) Update(tx *sql.Tx, integration *models.Integration) error {
	return lit.UpdateNamed(tx, integration, "id = :id", lit.P{"id": integration.Id})
}

func (r *integrationRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM integrations WHERE id = :id", lit.P{"id": id})
}

var IntegrationRepository = integrationRepository{}
