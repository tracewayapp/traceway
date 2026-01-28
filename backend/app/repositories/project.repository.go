package repositories

import (
	"backend/app/models"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/go-lightning/lit"
)

type projectRepository struct{}

type projectWithRole struct {
	Id             uuid.UUID `lit:"id"`
	Name           string    `lit:"name"`
	Token          string    `lit:"token"`
	Framework      string    `lit:"framework"`
	OrganizationId *int      `lit:"organization_id"`
	CreatedAt      time.Time `lit:"created_at"`
	Role           string    `lit:"role"`
}

func init() {
	lit.RegisterModel[projectWithRole](lit.PostgreSQL)
}

func (p *projectRepository) FindAllWithBackendUrlByUserId(tx *sql.Tx, userId int) ([]*models.ProjectWithBackendUrl, error) {
	rows, err := lit.Select[projectWithRole](
		tx,
		`SELECT DISTINCT p.id, p.name, p.token, p.framework, p.organization_id, p.created_at, ou.role
		FROM projects p
		INNER JOIN organization_users ou ON p.organization_id = ou.organization_id
		WHERE ou.user_id = $1
		ORDER BY p.created_at ASC`,
		userId,
	)
	if err != nil {
		return nil, err
	}

	result := make([]*models.ProjectWithBackendUrl, 0, len(rows))
	for _, row := range rows {
		token := row.Token
		if row.Role == "readonly" {
			token = "read-only-hidden-token"
		}

		project := models.Project{
			Id:             row.Id,
			Name:           row.Name,
			Token:          token,
			Framework:      row.Framework,
			OrganizationId: row.OrganizationId,
			CreatedAt:      row.CreatedAt,
		}
		result = append(result, project.ToProjectWithBackendUrl())
	}

	return result, nil
}

func (p *projectRepository) FindAll(tx *sql.Tx) ([]*models.Project, error) {
	return lit.Select[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at FROM projects ORDER BY created_at ASC",
	)
}

func (p *projectRepository) FindByToken(tx *sql.Tx, token string) (*models.Project, error) {
	return lit.SelectSingle[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at FROM projects WHERE token = $1",
		token,
	)
}

func (p *projectRepository) FindById(tx *sql.Tx, id uuid.UUID) (*models.Project, error) {
	return lit.SelectSingle[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at FROM projects WHERE id = $1",
		id,
	)
}

func (p *projectRepository) Create(tx *sql.Tx, name string, framework string) (*models.Project, error) {
	project := &models.Project{
		Id:        uuid.New(),
		Name:      name,
		Token:     generateSecureToken(),
		Framework: framework,
		CreatedAt: time.Now().UTC(),
	}

	err := lit.InsertExistingUuid(tx, project)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (p *projectRepository) CreateWithOrganization(tx *sql.Tx, name string, framework string, organizationId int) (*models.Project, error) {
	project := &models.Project{
		Id:             uuid.New(),
		Name:           name,
		Token:          generateSecureToken(),
		Framework:      framework,
		OrganizationId: &organizationId,
		CreatedAt:      time.Now().UTC(),
	}

	err := lit.InsertExistingUuid(tx, project)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (p *projectRepository) FindByOrganizationId(tx *sql.Tx, organizationId int) ([]*models.Project, error) {
	return lit.Select[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at FROM projects WHERE organization_id = $1 ORDER BY created_at ASC",
		organizationId,
	)
}

// FindByUserId returns all projects belonging to organizations the user is a member of
func (p *projectRepository) FindByUserId(tx *sql.Tx, userId int) ([]*models.Project, error) {
	return lit.Select[models.Project](
		tx,
		`SELECT DISTINCT p.id, p.name, p.token, p.framework, p.organization_id, p.created_at
		FROM projects p
		INNER JOIN organization_users ou ON p.organization_id = ou.organization_id
		WHERE ou.user_id = $1
		ORDER BY p.created_at ASC`,
		userId,
	)
}
func (p *projectRepository) UserHasAccess(tx *sql.Tx, projectId uuid.UUID, userId int) (bool, error) {
	result, err := lit.SelectSingle[models.CountResult](
		tx,
		`SELECT COUNT(*) as count
		FROM projects p
		INNER JOIN organization_users ou ON p.organization_id = ou.organization_id
		WHERE p.id = $1 AND ou.user_id = $2`,
		projectId,
		userId,
	)
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}

	return result.Count > 0, nil
}

func generateSecureToken() string {
	id := uuid.New()
	return strings.ReplaceAll(id.String(), "-", "")
}

var ProjectRepository = projectRepository{}
