//go:build !transactional_pg

package sqlite

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type repositoryRepository struct{}

const repositoryColumns = "id, project_id, integration_id, owner, name, default_branch, image, setup_command, test_command, created_at, updated_at"

func (r *repositoryRepository) Create(tx *sql.Tx, repository *models.Repository) (int, error) {
	return lit.Insert[models.Repository](tx, repository)
}

func (r *repositoryRepository) FindById(tx *sql.Tx, id int) (*models.Repository, error) {
	return lit.SelectSingleNamed[models.Repository](
		tx,
		"SELECT "+repositoryColumns+" FROM repositories WHERE id = :id",
		lit.P{"id": id},
	)
}

// FindByProject returns the project's one repository binding, or nil.
func (r *repositoryRepository) FindByProject(tx *sql.Tx, projectId uuid.UUID) (*models.Repository, error) {
	return lit.SelectSingleNamed[models.Repository](
		tx,
		"SELECT "+repositoryColumns+" FROM repositories WHERE project_id = :project_id",
		lit.P{"project_id": projectId},
	)
}

func (r *repositoryRepository) Update(tx *sql.Tx, repository *models.Repository) error {
	return lit.UpdateNamed(tx, repository, "id = :id", lit.P{"id": repository.Id})
}

func (r *repositoryRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM repositories WHERE id = :id", lit.P{"id": id})
}

var RepositoryRepository = repositoryRepository{}
