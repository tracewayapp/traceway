//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/services/contentflag"

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
	ProfileLabelAllowlist   models.StringSlice `lit:"profile_label_allowlist"`
	AiFlaggedTerms          models.StringSlice `lit:"ai_flagged_terms"`
	AiFlaggedLanguages      models.StringSlice `lit:"ai_flagged_languages"`
	Role                    string             `lit:"role"`
	OverrideRole            *string            `lit:"override_role"`
}

type effectiveRoleRow struct {
	OrgRole      string  `lit:"org_role"`
	OverrideRole *string `lit:"override_role"`
}

func init() {
	models.ExtensionModelRegistrations = append(models.ExtensionModelRegistrations, func(driver lit.Driver) {
		lit.RegisterModel[projectWithRole](driver)
		lit.RegisterModel[effectiveRoleRow](driver)
	})
}

func effectiveRole(orgRole string, overrideRole *string) string {
	if orgRole == "owner" || orgRole == "admin" {
		return orgRole
	}
	if overrideRole != nil {
		return *overrideRole
	}
	return orgRole
}

func (p *projectRepository) FindAllWithBackendUrlByUserId(tx *sql.Tx, userId int) ([]*models.ProjectWithBackendUrl, error) {
	rows, err := lit.SelectNamed[projectWithRole](
		tx,
		`SELECT DISTINCT p.id, p.name, p.token, p.framework, p.organization_id, p.created_at, p.source_map_token, p.drop_healthy_healthchecks, p.healthcheck_paths, p.profile_label_allowlist, p.ai_flagged_terms, p.ai_flagged_languages, ou.role, pur.role as override_role
		FROM projects p
		INNER JOIN organization_users ou ON p.organization_id = ou.organization_id
		LEFT JOIN project_user_roles pur ON pur.project_id = p.id AND pur.user_id = :user_id
		WHERE ou.user_id = :user_id
		ORDER BY p.created_at ASC`,
		lit.P{"user_id": userId},
	)
	if err != nil {
		return nil, err
	}

	result := make([]*models.ProjectWithBackendUrl, 0, len(rows))
	for _, row := range rows {
		role := effectiveRole(row.Role, row.OverrideRole)
		token := row.Token
		sourceMapToken := row.SourceMapToken
		if role == "readonly" {
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
			ProfileLabelAllowlist:   row.ProfileLabelAllowlist,
			AiFlaggedTerms:          row.AiFlaggedTerms,
			AiFlaggedLanguages:      row.AiFlaggedLanguages,
		}
		projectWithUrl := project.ToProjectWithBackendUrl()
		projectWithUrl.Role = role
		result = append(result, projectWithUrl)
	}

	return result, nil
}

func (p *projectRepository) GetEffectiveRole(tx *sql.Tx, projectId uuid.UUID, userId int) (string, error) {
	row, err := lit.SelectSingleNamed[effectiveRoleRow](
		tx,
		`SELECT ou.role as org_role, pur.role as override_role
		FROM projects p
		INNER JOIN organization_users ou ON ou.organization_id = p.organization_id AND ou.user_id = :user_id
		LEFT JOIN project_user_roles pur ON pur.project_id = p.id AND pur.user_id = :user_id
		WHERE p.id = :project_id`,
		lit.P{"user_id": userId, "project_id": projectId},
	)
	if err != nil {
		return "", err
	}
	if row == nil {
		return "", nil
	}
	return effectiveRole(row.OrgRole, row.OverrideRole), nil
}

func (p *projectRepository) FindAll(tx *sql.Tx) ([]*models.Project, error) {
	return lit.Select[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at, source_map_token, drop_healthy_healthchecks, healthcheck_paths, profile_label_allowlist, ai_flagged_terms, ai_flagged_languages FROM projects ORDER BY created_at ASC",
	)
}

func (p *projectRepository) FindByToken(tx *sql.Tx, token string) (*models.Project, error) {
	return lit.SelectSingleNamed[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at, source_map_token, drop_healthy_healthchecks, healthcheck_paths, profile_label_allowlist, ai_flagged_terms, ai_flagged_languages FROM projects WHERE token = :token",
		lit.P{"token": token},
	)
}

func (p *projectRepository) FindById(tx *sql.Tx, id uuid.UUID) (*models.Project, error) {
	return lit.SelectSingleNamed[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at, source_map_token, drop_healthy_healthchecks, healthcheck_paths, profile_label_allowlist, ai_flagged_terms, ai_flagged_languages FROM projects WHERE id = :id",
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
		AiFlaggedLanguages:      models.StringSlice(contentflag.DefaultLanguages),
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
		ProfileLabelAllowlist:   models.StringSlice{},
		AiFlaggedTerms:          models.StringSlice{},
		AiFlaggedLanguages:      models.StringSlice(contentflag.DefaultLanguages),
	}

	if frameworkRequiresSymbolUpload(framework) {
		token := generateSecureToken()
		project.SourceMapToken = &token
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
		"SELECT id, name, token, framework, organization_id, created_at, source_map_token, drop_healthy_healthchecks, healthcheck_paths, profile_label_allowlist, ai_flagged_terms, ai_flagged_languages FROM projects WHERE organization_id = :org_id ORDER BY created_at ASC",
		lit.P{"org_id": organizationId},
	)
}

func (p *projectRepository) FindByUserId(tx *sql.Tx, userId int) ([]*models.Project, error) {
	return lit.SelectNamed[models.Project](
		tx,
		`SELECT DISTINCT p.id, p.name, p.token, p.framework, p.organization_id, p.created_at, p.source_map_token, p.drop_healthy_healthchecks, p.healthcheck_paths, p.profile_label_allowlist, p.ai_flagged_terms, p.ai_flagged_languages
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

func (p *projectRepository) Update(tx *sql.Tx, id uuid.UUID, name string, framework string, dropHealthyHealthchecks *bool, healthcheckPaths *[]string, profileLabelAllowlist *[]string, aiFlaggedTerms *[]string, aiFlaggedLanguages *[]string) (*models.Project, error) {
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
	if profileLabelAllowlist != nil {
		project.ProfileLabelAllowlist = models.StringSlice(*profileLabelAllowlist)
	}
	if aiFlaggedTerms != nil {
		project.AiFlaggedTerms = models.StringSlice(*aiFlaggedTerms)
	}
	if aiFlaggedLanguages != nil {
		project.AiFlaggedLanguages = models.StringSlice(*aiFlaggedLanguages)
	}
	err = lit.UpdateNamed[models.Project](tx, project, "id = :id", lit.P{"id": id})
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (p *projectRepository) Delete(tx *sql.Tx, id uuid.UUID) error {
	related := []string{
		"notification_rules",
		"notification_channels",
		"widget_groups",
		"project_dashboards",
		"starred_dashboard_widgets",
		"source_maps",
		"metric_registry",
		"project_user_roles",
	}
	for _, table := range related {
		if err := lit.DeleteNamed(db.Driver, tx, "DELETE FROM "+table+" WHERE project_id = :project_id", lit.P{"project_id": id}); err != nil {
			return fmt.Errorf("deleting %s: %w", table, err)
		}
	}
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM projects WHERE id = :id", lit.P{"id": id})
}

func (p *projectRepository) FindBySourceMapToken(tx *sql.Tx, token string) (*models.Project, error) {
	return lit.SelectSingleNamed[models.Project](
		tx,
		"SELECT id, name, token, framework, organization_id, created_at, source_map_token, drop_healthy_healthchecks, healthcheck_paths, profile_label_allowlist, ai_flagged_terms, ai_flagged_languages FROM projects WHERE source_map_token = :smt",
		lit.P{"smt": token},
	)
}

func generateSecureToken() string {
	id := uuid.New()
	return strings.ReplaceAll(id.String(), "-", "")
}

func frameworkRequiresSymbolUpload(framework string) bool {
	switch framework {
	case "ios":
		return true
	default:
		return false
	}
}

var ProjectRepository = projectRepository{}
