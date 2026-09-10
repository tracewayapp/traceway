//go:build !transactional_pg

package sqlite

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type identityRepository struct{}

const identityColumns = "id, user_id, provider, external_id, display, created_at"

func (r *identityRepository) Create(tx *sql.Tx, identity *models.Identity) (int, error) {
	return lit.Insert[models.Identity](tx, identity)
}

// FindByExternalId resolves an external account to its mapping, the one
// authorization rule every channel and trigger applies to an inbound author.
func (r *identityRepository) FindByExternalId(tx *sql.Tx, provider string, externalId string) (*models.Identity, error) {
	return lit.SelectSingleNamed[models.Identity](
		tx,
		"SELECT "+identityColumns+" FROM identities WHERE provider = :provider AND external_id = :external_id",
		lit.P{"provider": provider, "external_id": externalId},
	)
}

func (r *identityRepository) FindByUser(tx *sql.Tx, userId int) ([]*models.Identity, error) {
	return lit.SelectNamed[models.Identity](
		tx,
		"SELECT "+identityColumns+" FROM identities WHERE user_id = :user_id ORDER BY provider ASC, id ASC",
		lit.P{"user_id": userId},
	)
}

func (r *identityRepository) FindByUserAndProvider(tx *sql.Tx, userId int, provider string) (*models.Identity, error) {
	return lit.SelectSingleNamed[models.Identity](
		tx,
		"SELECT "+identityColumns+" FROM identities WHERE user_id = :user_id AND provider = :provider ORDER BY id ASC LIMIT 1",
		lit.P{"user_id": userId, "provider": provider},
	)
}

func (r *identityRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM identities WHERE id = :id", lit.P{"id": id})
}

var IdentityRepository = identityRepository{}
