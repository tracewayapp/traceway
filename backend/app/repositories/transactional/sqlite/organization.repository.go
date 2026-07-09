//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type organizationRepository struct{}

func (r *organizationRepository) Create(tx *sql.Tx, name string, timezone string) (*models.Organization, error) {
	org := &models.Organization{
		Name:      name,
		Timezone:  timezone,
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

func (r *organizationRepository) FindFirst(tx *sql.Tx) (*models.Organization, error) {
	return lit.SelectSingle[models.Organization](
		tx,
		"SELECT id, name, timezone, created_at FROM organizations ORDER BY created_at ASC LIMIT 1",
	)
}

func (r *organizationRepository) FindByName(tx *sql.Tx, name string) (*models.Organization, error) {
	return lit.SelectSingleNamed[models.Organization](
		tx,
		"SELECT id, name, timezone, created_at FROM organizations WHERE name = :name LIMIT 1",
		lit.P{"name": name},
	)
}

func (r *organizationRepository) FindById(tx *sql.Tx, id int) (*models.Organization, error) {
	return lit.SelectSingleNamed[models.Organization](
		tx,
		"SELECT id, name, timezone, created_at FROM organizations WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *organizationRepository) FindByUserId(tx *sql.Tx, userId int) ([]*models.Organization, error) {
	return lit.SelectNamed[models.Organization](
		tx,
		`SELECT o.id, o.name, o.created_at
		FROM organizations o
		INNER JOIN organization_users ou ON o.id = ou.organization_id
		WHERE ou.user_id = :user_id
		ORDER BY o.created_at ASC`,
		lit.P{"user_id": userId},
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
	orgUser, err := lit.SelectSingleNamed[models.OrganizationUser](
		tx,
		"SELECT id, user_id, organization_id, role, created_at FROM organization_users WHERE organization_id = :org_id AND user_id = :user_id",
		lit.P{"org_id": organizationId, "user_id": userId},
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
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		`SELECT COUNT(*) as count FROM organization_users WHERE organization_id = :org_id`,
		lit.P{"org_id": organizationId},
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
	return lit.SelectNamed[models.OrganizationMember](
		tx,
		`SELECT u.id, u.email, u.name, ou.role, ou.created_at
		FROM users u
		JOIN organization_users ou ON u.id = ou.user_id
		WHERE ou.organization_id = :org_id
		ORDER BY ou.created_at ASC`,
		lit.P{"org_id": organizationId},
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
	q, a, err := lit.ParseNamedQuery(db.Driver, "UPDATE organization_users SET role = :role WHERE organization_id = :org_id AND user_id = :user_id", lit.P{"role": role, "org_id": organizationId, "user_id": userId})
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, q, a...)
}

func (r *organizationRepository) RemoveUser(tx *sql.Tx, organizationId int, userId int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM organization_users WHERE organization_id = :org_id AND user_id = :user_id", lit.P{"org_id": organizationId, "user_id": userId})
}

func (r *organizationRepository) IsUserMember(tx *sql.Tx, organizationId int, userId int) (bool, error) {
	role, err := r.GetUserRole(tx, organizationId, userId)
	if err != nil {
		return false, err
	}
	return role != "", nil
}

func (r *organizationRepository) IsUserMemberByEmail(tx *sql.Tx, organizationId int, email string) (bool, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		`SELECT COUNT(*) as count
		FROM organization_users ou
		JOIN users u ON u.id = ou.user_id
		WHERE ou.organization_id = :org_id AND u.email = :email`,
		lit.P{"org_id": organizationId, "email": email},
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
	return lit.SelectNamed[models.UserOrganizationResponse](
		tx,
		`SELECT o.id, o.name, ou.role, o.timezone
		FROM organizations o
		INNER JOIN organization_users ou ON o.id = ou.organization_id
		WHERE ou.user_id = :user_id
		ORDER BY o.created_at ASC`,
		lit.P{"user_id": userId},
	)
}

func (r *organizationRepository) UpdateTimezone(tx *sql.Tx, organizationId int, timezone string) error {
	q, a, err := lit.ParseNamedQuery(db.Driver, "UPDATE organizations SET timezone = :timezone WHERE id = :id", lit.P{"timezone": timezone, "id": organizationId})
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, q, a...)
}

var OrganizationRepository = organizationRepository{}
