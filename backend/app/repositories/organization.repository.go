package repositories

import (
	"backend/app/models"
	"database/sql"
	"time"

	"github.com/tracewayapp/go-lightning/lit"
)

type organizationRepository struct{}

func (r *organizationRepository) Create(tx *sql.Tx, name string) (*models.Organization, error) {
	org := &models.Organization{
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}

	id, err := lit.Insert(tx, org)
	if err != nil {
		return nil, err
	}
	org.Id = id

	return org, nil
}

func (r *organizationRepository) HasOrganizations(tx *sql.Tx) (bool, error) {
	result, err := lit.SelectSingle[models.CountResult](
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
		"SELECT id, name, timezone, created_at FROM organizations WHERE id = $1",
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
		CreatedAt:      time.Now().UTC(),
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

func (r *organizationRepository) CountMembers(tx *sql.Tx, organizationId int) (int, error) {
	result, err := lit.SelectSingle[models.CountResult](
		tx,
		`SELECT COUNT(*) as count FROM organization_users WHERE organization_id = $1`,
		organizationId,
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.Count, nil
}

func (r *organizationRepository) GetMembersWithDetails(tx *sql.Tx, organizationId int) ([]*models.OrganizationMember, error) {
	return lit.Select[models.OrganizationMember](
		tx,
		`SELECT u.id, u.email, u.name, ou.role, ou.created_at
		FROM users u
		JOIN organization_users ou ON u.id = ou.user_id
		WHERE ou.organization_id = $1
		ORDER BY ou.created_at ASC`,
		organizationId,
	)
}

func (r *organizationRepository) IsOwner(tx *sql.Tx, organizationId int, userId int) (bool, error) {
	role, err := r.GetUserRole(tx, organizationId, userId)
	if err != nil {
		return false, err
	}
	return role == "owner", nil
}

func (r *organizationRepository) UpdateUserRole(tx *sql.Tx, organizationId int, userId int, role string) error {
	return lit.UpdateNative(
		tx,
		"UPDATE organization_users SET role = $1 WHERE organization_id = $2 AND user_id = $3",
		role,
		organizationId,
		userId,
	)
}

func (r *organizationRepository) RemoveUser(tx *sql.Tx, organizationId int, userId int) error {
	return lit.Delete(tx, "DELETE FROM organization_users WHERE organization_id = $1 AND user_id = $2", organizationId, userId)
}

func (r *organizationRepository) IsUserMember(tx *sql.Tx, organizationId int, userId int) (bool, error) {
	role, err := r.GetUserRole(tx, organizationId, userId)
	if err != nil {
		return false, err
	}
	return role != "", nil
}

func (r *organizationRepository) IsUserMemberByEmail(tx *sql.Tx, organizationId int, email string) (bool, error) {
	result, err := lit.SelectSingle[models.CountResult](
		tx,
		`SELECT COUNT(*) as count
		FROM organization_users ou
		JOIN users u ON u.id = ou.user_id
		WHERE ou.organization_id = $1 AND u.email = $2`,
		organizationId,
		email,
	)
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}
	return result.Count > 0, nil
}

func (r *organizationRepository) FindByUserIdWithRoles(tx *sql.Tx, userId int) ([]*models.UserOrganizationResponse, error) {
	return lit.Select[models.UserOrganizationResponse](
		tx,
		`SELECT o.id, o.name, ou.role, o.timezone
		FROM organizations o
		INNER JOIN organization_users ou ON o.id = ou.organization_id
		WHERE ou.user_id = $1
		ORDER BY o.created_at ASC`,
		userId,
	)
}

func (r *organizationRepository) UpdateTimezone(tx *sql.Tx, organizationId int, timezone string) error {
	return lit.UpdateNative(
		tx,
		"UPDATE organizations SET timezone = $1 WHERE id = $2",
		timezone,
		organizationId,
	)
}

var OrganizationRepository = organizationRepository{}
