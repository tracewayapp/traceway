//go:build !transactional_pg

package sqlite

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

type savedFilterRepository struct{}

var SavedFilterRepository = savedFilterRepository{}

const savedFilterColumns = "id, project_id, page, name, query, created_by, created_at, updated_at"

func (r *savedFilterRepository) List(tx *sql.Tx, projectId uuid.UUID, page string) ([]*models.SavedFilterListItem, error) {
	return lit.SelectNamed[models.SavedFilterListItem](
		tx,
		"SELECT sf.id, sf.project_id, sf.page, sf.name, sf.query, sf.created_by, sf.created_at, sf.updated_at, u.name AS created_by_name"+
			" FROM saved_filters sf LEFT JOIN users u ON u.id = sf.created_by"+
			" WHERE sf.project_id = :project_id AND sf.page = :page ORDER BY sf.name ASC, sf.id ASC",
		lit.P{"project_id": projectId, "page": page},
	)
}

func (r *savedFilterRepository) Find(tx *sql.Tx, projectId uuid.UUID, id int) (*models.SavedFilter, error) {
	return lit.SelectSingleNamed[models.SavedFilter](
		tx,
		"SELECT "+savedFilterColumns+" FROM saved_filters WHERE id = :id AND project_id = :project_id",
		lit.P{"id": id, "project_id": projectId},
	)
}

func (r *savedFilterRepository) Create(tx *sql.Tx, filter *models.SavedFilter) (int, error) {
	return lit.Insert[models.SavedFilter](tx, filter)
}

func (r *savedFilterRepository) Update(tx *sql.Tx, filter *models.SavedFilter) error {
	return lit.UpdateNamed(tx, filter, "id = :id AND project_id = :project_id", lit.P{"id": filter.Id, "project_id": filter.ProjectId})
}

func (r *savedFilterRepository) Delete(tx *sql.Tx, projectId uuid.UUID, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM saved_filters WHERE id = :id AND project_id = :project_id", lit.P{"id": id, "project_id": projectId})
}
