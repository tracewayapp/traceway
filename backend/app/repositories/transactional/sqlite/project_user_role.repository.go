//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type projectUserRoleRepository struct{}

func (r *projectUserRoleRepository) FindByOrganizationAndUser(tx *sql.Tx, organizationId int, userId int) ([]*models.MemberProjectRole, error) {
	return lit.SelectNamed[models.MemberProjectRole](
		tx,
		`SELECT p.id as project_id, p.name, p.framework, pur.role
		FROM projects p
		LEFT JOIN project_user_roles pur ON pur.project_id = p.id AND pur.user_id = :user_id
		WHERE p.organization_id = :org_id
		ORDER BY p.created_at ASC`,
		lit.P{"org_id": organizationId, "user_id": userId},
	)
}

func (r *projectUserRoleRepository) Upsert(tx *sql.Tx, projectId uuid.UUID, userId int, role string) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO project_user_roles (project_id, user_id, role, created_at)
		VALUES (:project_id, :user_id, :role, :created_at)
		ON CONFLICT (project_id, user_id) DO UPDATE SET role = :role`,
		lit.P{
			"project_id": projectId,
			"user_id":    userId,
			"role":       role,
			"created_at": time.Now().UTC(),
		},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *projectUserRoleRepository) Delete(tx *sql.Tx, projectId uuid.UUID, userId int) error {
	return lit.DeleteNamed(
		db.Driver,
		tx,
		"DELETE FROM project_user_roles WHERE project_id = :project_id AND user_id = :user_id",
		lit.P{"project_id": projectId, "user_id": userId},
	)
}

func (r *projectUserRoleRepository) DeleteByOrganizationAndUser(tx *sql.Tx, organizationId int, userId int) error {
	return lit.DeleteNamed(
		db.Driver,
		tx,
		`DELETE FROM project_user_roles
		WHERE user_id = :user_id
		AND project_id IN (SELECT id FROM projects WHERE organization_id = :org_id)`,
		lit.P{"user_id": userId, "org_id": organizationId},
	)
}

var ProjectUserRoleRepository = projectUserRoleRepository{}
