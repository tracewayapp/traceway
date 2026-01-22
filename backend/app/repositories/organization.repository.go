package repositories

import (
	"backend/app/models"
	"database/sql"

	"github.com/tracewayapp/go-lightning/lit"
)

type organizationRepository struct{}

func (r *organizationRepository) Create(tx *sql.Tx, name string) (*models.Organization, error) {
	org := &models.Organization{
		Name: name,
	}

	id, err := lit.Insert(tx, org)
	if err != nil {
		return nil, err
	}
	org.Id = id

	return org, nil
}

func (r *organizationRepository) HasOrganizations(tx *sql.Tx) (bool, error) {
	result, err := lit.SelectSingle[CountResult](
		tx,
		`SELECT COUNT(*) as count
		FROM organizations`,
	)
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}

	return result.Count > 0, nil
}

func (r *organizationRepository) FindById(tx *sql.Tx, id int) (*models.Organization, error) {
	return lit.SelectSingle[models.Organization](
		tx,
		"SELECT id, name, created_at FROM organizations WHERE id = $1",
		id,
	)
}

func (r *organizationRepository) FindByUserId(tx *sql.Tx, userId int) ([]*models.Organization, error) {
	return lit.Select[models.Organization](
		tx,
		`SELECT o.id, o.name, o.created_at
		FROM organizations o
		INNER JOIN organization_users ou ON o.id = ou.organization_id
		WHERE ou.user_id = $1
		ORDER BY o.created_at ASC`,
		userId,
	)
}

func (r *organizationRepository) AddUser(tx *sql.Tx, organizationId int, userId int, role string) (*models.OrganizationUser, error) {
	orgUser := &models.OrganizationUser{
		UserId:         userId,
		OrganizationId: organizationId,
		Role:           role,
	}

	id, err := lit.Insert(tx, orgUser)
	if err != nil {
		return nil, err
	}
	orgUser.Id = id

	return orgUser, nil
}

func (r *organizationRepository) GetUserRole(tx *sql.Tx, organizationId int, userId int) (string, error) {
	orgUser, err := lit.SelectSingle[models.OrganizationUser](
		tx,
		"SELECT id, user_id, organization_id, role, created_at FROM organization_users WHERE organization_id = $1 AND user_id = $2",
		organizationId,
		userId,
	)
	if err != nil {
		return "", err
	}
	if orgUser == nil {
		return "", nil // User not in organization
	}
	return orgUser.Role, nil
}

var OrganizationRepository = organizationRepository{}
