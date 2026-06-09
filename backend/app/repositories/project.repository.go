package repositories

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type projectRepository struct{}

type projectWithRole struct {
	Id                      uuid.UUID          `lit:"id"`
	Name                    string             `lit:"name"`
	Token                   string             `lit:"token"`
	Framework               string             `lit:"framework"`
	OrganizationId          *int               `lit:"organization_id"`
	CreatedAt               time.Time          `lit:"created_at"`
	SourceMapToken          *string            `lit:"source_map_token"`
	DropHealthyHealthchecks bool               `lit:"drop_healthy_healthchecks"`
	HealthcheckPaths        models.StringSlice `lit:"healthcheck_paths"`
	Role                    string             `lit:"role"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[projectWithRole](driver)
	})
}

func (p *projectRepository) FindAllWithBackendUrlByUserId(tx *sql.Tx, userId int) ([]*models.ProjectWithBackendUrl, error) {
	rows, err := lit.SelectNamed[projectWithRole](
		tx,
		`SELECT DISTINCT p.id, p.name, p.token, p.framework, p.organization_id, p.created_at, p.source_map_token, p.drop_healthy_healthchecks, p.healthcheck_paths, ou.role
		FROM projects p
		INNER JOIN organization_users ou ON p.organization_id = ou.organization_id
		WHERE ou.user_id = :user_id
		ORDER BY p.created_at ASC`,
		lit.P{"user_id": userId},
	)
	if err != nil {
		return nil, err
	}

	result := make([]*models.ProjectWithBackendUrl, 0, len(rows))
	for _, row := range rows {
		token := row.Token
		sourceMapToken := row.SourceMapToken
		if row.Role == "readonly" {
			token = "read-only-hidden-token"
			sourceMapToken = nil
		}

		project := models.Project{
			Id:                      row.Id,
			Name:                    row.Name,
			Token:                   token,
			Framework:               row.Framework,
			OrganizationId:          row.OrganizationId,
			CreatedAt:               row.CreatedAt,
			SourceMapToken:          sourceMapToken,
			DropHealthyHealthchecks: row.DropHealthyHealthchecks,
			HealthcheckPaths:        row.HealthcheckPaths,
		}
		result = append(result, project.ToProjectWithBackendUrl())
	}

	return result, nil
}

func (p *projectRepository) FindAll(tx *sql.Tx) ([]*models.Project, error) {
	return lit.Select[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at, source_map_token, drop_healthy_healthchecks, healthcheck_paths FROM projects ORDER BY created_at ASC",
	)
}

func (p *projectRepository) FindByToken(tx *sql.Tx, token string) (*models.Project, error) {
	return lit.SelectSingleNamed[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at, source_map_token, drop_healthy_healthchecks, healthcheck_paths FROM projects WHERE token = :token",
		lit.P{"token": token},
	)
}

func (p *projectRepository) FindById(tx *sql.Tx, id uuid.UUID) (*models.Project, error) {
	return lit.SelectSingleNamed[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at, source_map_token, drop_healthy_healthchecks, healthcheck_paths FROM projects WHERE id = :id",
		lit.P{"id": id},
	)
}

func (p *projectRepository) Create(tx *sql.Tx, name string, framework string) (*models.Project, error) {
	project := &models.Project{
		Id:                      uuid.New(),
		Name:                    name,
		Token:                   generateSecureToken(),
		Framework:               framework,
		CreatedAt:               time.Now().UTC(),
		DropHealthyHealthchecks: true,
	}

	err := lit.InsertExistingUuid(tx, project)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (p *projectRepository) CreateWithOrganization(tx *sql.Tx, name string, framework string, organizationId int) (*models.Project, error) {
	project := &models.Project{
		Id:                      uuid.New(),
		Name:                    name,
		Token:                   generateSecureToken(),
		Framework:               framework,
		OrganizationId:          &organizationId,
		CreatedAt:               time.Now().UTC(),
		DropHealthyHealthchecks: true,
	}

	err := lit.InsertExistingUuid(tx, project)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (p *projectRepository) FindByOrganizationId(tx *sql.Tx, organizationId int) ([]*models.Project, error) {
	return lit.SelectNamed[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at, source_map_token, drop_healthy_healthchecks, healthcheck_paths FROM projects WHERE organization_id = :org_id ORDER BY created_at ASC",
		lit.P{"org_id": organizationId},
	)
}

func (p *projectRepository) FindByUserId(tx *sql.Tx, userId int) ([]*models.Project, error) {
	return lit.SelectNamed[models.Project](
		tx,
		`SELECT DISTINCT p.id, p.name, p.token, p.framework, p.organization_id, p.created_at, p.source_map_token, p.drop_healthy_healthchecks, p.healthcheck_paths
		FROM projects p
		INNER JOIN organization_users ou ON p.organization_id = ou.organization_id
		WHERE ou.user_id = :user_id
		ORDER BY p.created_at ASC`,
		lit.P{"user_id": userId},
	)
}

func (p *projectRepository) UserHasAccess(tx *sql.Tx, projectId uuid.UUID, userId int) (bool, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		`SELECT COUNT(*) as count
		FROM projects p
		INNER JOIN organization_users ou ON p.organization_id = ou.organization_id
		WHERE p.id = :project_id AND ou.user_id = :user_id`,
		lit.P{"project_id": projectId, "user_id": userId},
	)
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}

	return result.Count > 0, nil
}

func (p *projectRepository) GenerateSourceMapToken(tx *sql.Tx, projectId uuid.UUID) (string, error) {
	project, err := p.FindById(tx, projectId)
	if err != nil {
		return "", err
	}
	if project == nil {
		return "", fmt.Errorf("project not found: %s", projectId)
	}

	token := generateSecureToken()
	project.SourceMapToken = &token
	err = lit.UpdateNamed[models.Project](tx, project, "id = :id", lit.P{"id": projectId})
	if err != nil {
		return "", err
	}
	return token, nil
}

func (p *projectRepository) Update(tx *sql.Tx, id uuid.UUID, name string, framework string, dropHealthyHealthchecks *bool, healthcheckPaths *[]string) (*models.Project, error) {
	project, err := p.FindById(tx, id)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, fmt.Errorf("project not found: %s", id)
	}
	project.Name = name
	project.Framework = framework
	if dropHealthyHealthchecks != nil {
		project.DropHealthyHealthchecks = *dropHealthyHealthchecks
	}
	if healthcheckPaths != nil {
		project.HealthcheckPaths = models.StringSlice(*healthcheckPaths)
	}
	err = lit.UpdateNamed[models.Project](tx, project, "id = :id", lit.P{"id": id})
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (p *projectRepository) Delete(tx *sql.Tx, id uuid.UUID) error {
	related := []string{
		"notification_history",
		"notification_rules",
		"notification_channels",
		"widget_groups",
		"source_maps",
		"metric_registry",
	}
	for _, table := range related {
		if err := lit.Delete(tx, "DELETE FROM "+table+" WHERE project_id = $1", id); err != nil {
			return fmt.Errorf("deleting %s: %w", table, err)
		}
	}
	return lit.Delete(tx, "DELETE FROM projects WHERE id = $1", id)
}

func (p *projectRepository) FindBySourceMapToken(tx *sql.Tx, token string) (*models.Project, error) {
	return lit.SelectSingleNamed[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at, source_map_token, drop_healthy_healthchecks, healthcheck_paths FROM projects WHERE source_map_token = :smt",
		lit.P{"smt": token},
	)
}

func generateSecureToken() string {
	id := uuid.New()
	return strings.ReplaceAll(id.String(), "-", "")
}

var ProjectRepository = projectRepository{}
