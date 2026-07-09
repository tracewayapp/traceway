//go:build transactional_pg

package pg

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type invitationRepository struct{}

func (r *invitationRepository) Create(tx *sql.Tx, organizationId int, email string, role string, invitedBy int, expiresAt time.Time) (*models.Invitation, error) {
	token := uuid.New().String()

	invitation := &models.Invitation{
		OrganizationId: organizationId,
		Email:          email,
		Role:           role,
		Token:          token,
		InvitedBy:      invitedBy,
		Status:         "pending",
		ExpiresAt:      expiresAt,
		CreatedAt:      time.Now().UTC(),
	}

	id, err := lit.Insert(tx, invitation)
	if err != nil {
		return nil, err
	}
	invitation.Id = id

	return invitation, nil
}

func (r *invitationRepository) FindByToken(tx *sql.Tx, token string) (*models.Invitation, error) {
	return lit.SelectSingleNamed[models.Invitation](
		tx,
		`SELECT id, organization_id, email, role, token, invited_by, status, expires_at, accepted_at, created_at
		FROM invitations
		WHERE token = :token`,
		lit.P{"token": token},
	)
}

func (r *invitationRepository) FindByOrganization(tx *sql.Tx, organizationId int) ([]*models.InvitationWithInviter, error) {
	return lit.SelectNamed[models.InvitationWithInviter](
		tx,
		`SELECT i.id, i.organization_id, i.email, i.role, i.invited_by, u.name as inviter_name, i.status, i.expires_at, i.accepted_at, i.created_at
		FROM invitations i
		JOIN users u ON i.invited_by = u.id
		WHERE i.organization_id = :org_id AND i.status = 'pending'
		ORDER BY i.created_at DESC`,
		lit.P{"org_id": organizationId},
	)
}

func (r *invitationRepository) Update(tx *sql.Tx, invitation *models.Invitation) error {
	return lit.UpdateNamed[models.Invitation](
		tx,
		invitation,
		"id = :id",
		lit.P{"id": invitation.Id},
	)
}

func (r *invitationRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM invitations WHERE id = :id", lit.P{"id": id})
}

func (r *invitationRepository) HasPendingInvitation(tx *sql.Tx, email string, organizationId int) (bool, error) {
	invitation, err := lit.SelectSingleNamed[models.Invitation](
		tx,
		`SELECT id, organization_id, email, role, token, invited_by, status, expires_at, accepted_at, created_at
		FROM invitations
		WHERE email = :email AND organization_id = :org_id AND status = 'pending'`,
		lit.P{"email": email, "org_id": organizationId},
	)
	if err != nil {
		return false, err
	}
	return invitation != nil, nil
}

func (r *invitationRepository) FindById(tx *sql.Tx, id int) (*models.Invitation, error) {
	return lit.SelectSingleNamed[models.Invitation](
		tx,
		`SELECT id, organization_id, email, role, token, invited_by, status, expires_at, accepted_at, created_at
		FROM invitations
		WHERE id = :id`,
		lit.P{"id": id},
	)
}

var InvitationRepository = invitationRepository{}
