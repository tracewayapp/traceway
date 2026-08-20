//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"errors"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
)

type setupTokenRepository struct{}

func (r *setupTokenRepository) Create(ex lit.Executor, id, token, prefix string, userId, organizationId int, expiresAt time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO setup_tokens (id, token_hash, prefix, user_id, organization_id, created_at, expires_at)
			VALUES (:id, :hash, :prefix, :user_id, :organization_id, :created_at, :expires_at)`,
		lit.P{
			"id":              id,
			"hash":            shared.HashAuthToken(token),
			"prefix":          prefix,
			"user_id":         userId,
			"organization_id": organizationId,
			"created_at":      shared.FormatAuthTime(time.Now()),
			"expires_at":      shared.FormatAuthTime(expiresAt),
		},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

// DeleteByUserAndOrganization enforces single-active-token-per-(user, org):
// minting a new setup token replaces any previous one.
func (r *setupTokenRepository) DeleteByUserAndOrganization(ex lit.Executor, userId, organizationId int) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM setup_tokens WHERE user_id = :user_id AND organization_id = :org_id",
		lit.P{"user_id": userId, "org_id": organizationId},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

// FindActiveByToken returns nil for unknown or expired tokens.
func (r *setupTokenRepository) FindActiveByToken(ex lit.Executor, token string) (*shared.ActiveSetupToken, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`SELECT s.id, s.user_id, s.organization_id, s.expires_at, u.email
			FROM setup_tokens s
			JOIN users u ON u.id = s.user_id
			JOIN organization_users ou
				ON ou.user_id = s.user_id
				AND ou.organization_id = s.organization_id
				AND ou.role <> 'readonly'
			WHERE s.token_hash = :hash`,
		lit.P{"hash": shared.HashAuthToken(token)},
	)
	if err != nil {
		return nil, err
	}

	var (
		st        shared.ActiveSetupToken
		expiresAt string
	)
	err = ex.QueryRow(query, args...).Scan(&st.Id, &st.UserId, &st.OrganizationId, &expiresAt, &st.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if st.ExpiresAt, err = shared.ParseAuthTime(expiresAt); err != nil {
		return nil, err
	}
	if time.Now().After(st.ExpiresAt) {
		return nil, nil
	}
	return &st, nil
}

func (r *setupTokenRepository) PruneExpired(ex lit.Executor, now time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM setup_tokens WHERE expires_at < :cutoff",
		lit.P{"cutoff": shared.FormatAuthTime(now)},
	)
	if err != nil {
		return 0, err
	}
	res, err := ex.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

var SetupTokenRepository = setupTokenRepository{}
