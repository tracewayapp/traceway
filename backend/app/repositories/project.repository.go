package repositories

import (
	"backend/app/models"
	"database/sql"
	"strings"

	"github.com/google/uuid"
	"github.com/tracewayapp/go-lightning/lit"
)

type projectRepository struct{}

func (p *projectRepository) FindAll(tx *sql.Tx) ([]*models.Project, error) {
	return lit.Select[models.Project](
		tx,
		"SELECT id, name, token, framework, created_at FROM projects ORDER BY created_at ASC",
	)
}

func (p *projectRepository) FindByToken(tx *sql.Tx, token string) (*models.Project, error) {
	return lit.SelectSingle[models.Project](
		tx,
		"SELECT id, name, token, framework, created_at FROM projects WHERE token = $1",
		token,
	)
}

func (p *projectRepository) FindById(tx *sql.Tx, id uuid.UUID) (*models.Project, error) {
	return lit.SelectSingle[models.Project](
		tx,
		"SELECT id, name, token, framework, created_at FROM projects WHERE id = $1",
		id,
	)
}

func (p *projectRepository) Create(tx *sql.Tx, name string, framework string) (*models.Project, error) {
	project := &models.Project{
		Id:        uuid.New(),
		Name:      name,
		Token:     generateSecureToken(),
		Framework: framework,
	}

	err := lit.InsertExistingUuid(tx, project)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func generateSecureToken() string {
	id := uuid.New()
	return strings.ReplaceAll(id.String(), "-", "")
}

var ProjectRepository = projectRepository{}
