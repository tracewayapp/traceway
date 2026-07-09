//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type personalAccessTokenRepository struct{}

func (r *personalAccessTokenRepository) Create(ex lit.Executor, id, token, prefix string, userId int, name string, expiresAt *time.Time) error {
	var expires any
	if expiresAt != nil {
		expires = shared.FormatAuthTime(*expiresAt)
	}
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO personal_access_tokens (id, token_hash, prefix, user_id, name, revoked, expires_at, created_at)
			VALUES (:id, :hash, :prefix, :user_id, :name, :revoked, :expires_at, :created_at)`,
		lit.P{
			"id":         id,
			"hash":       shared.HashAuthToken(token),
			"prefix":     prefix,
			"user_id":    userId,
			"name":       name,
			"revoked":    false,
			"expires_at": expires,
			"created_at": shared.FormatAuthTime(time.Now()),
		},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

func (r *personalAccessTokenRepository) FindActiveByToken(ex lit.Executor, token string) (*shared.ActivePAT, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`SELECT p.id, p.user_id, p.revoked, p.expires_at, p.last_used_at, u.email
			FROM personal_access_tokens p
			JOIN users u ON u.id = p.user_id
			WHERE p.token_hash = :hash`,
		lit.P{"hash": shared.HashAuthToken(token)},
	)
	if err != nil {
		return nil, err
	}

	var (
		pat       shared.ActivePAT
		revoked   bool
		expiresAt sql.NullString
		lastUsed  sql.NullString
	)
	err = ex.QueryRow(query, args...).Scan(&pat.Id, &pat.UserId, &revoked, &expiresAt, &lastUsed, &pat.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if revoked {
		return nil, nil
	}
	expiry, err := shared.ParseNullableAuthTime(expiresAt)
	if err != nil {
		return nil, err
	}
	if expiry != nil && time.Now().After(*expiry) {
		return nil, nil
	}
	if pat.LastUsedAt, err = shared.ParseNullableAuthTime(lastUsed); err != nil {
		return nil, err
	}
	return &pat, nil
}

func (r *personalAccessTokenRepository) ListByUser(ex lit.Executor, userId int) ([]*models.PersonalAccessToken, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`SELECT id, prefix, name, last_used_at, expires_at, created_at
			FROM personal_access_tokens
			WHERE user_id = :user_id AND revoked = :revoked
			ORDER BY created_at DESC`,
		lit.P{"user_id": userId, "revoked": false},
	)
	if err != nil {
		return nil, err
	}

	rows, err := ex.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := []*models.PersonalAccessToken{}
	for rows.Next() {
		var (
			pat       models.PersonalAccessToken
			lastUsed  sql.NullString
			expiresAt sql.NullString
			createdAt string
		)
		if err := rows.Scan(&pat.Id, &pat.Prefix, &pat.Name, &lastUsed, &expiresAt, &createdAt); err != nil {
			return nil, err
		}
		if pat.LastUsedAt, err = shared.ParseNullableAuthTime(lastUsed); err != nil {
			return nil, err
		}
		if pat.ExpiresAt, err = shared.ParseNullableAuthTime(expiresAt); err != nil {
			return nil, err
		}
		if pat.CreatedAt, err = shared.ParseAuthTime(createdAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, &pat)
	}
	return tokens, rows.Err()
}

func (r *personalAccessTokenRepository) Revoke(ex lit.Executor, id string, userId int) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE personal_access_tokens SET revoked = :revoked WHERE id = :id AND user_id = :user_id AND revoked = :active",
		lit.P{"revoked": true, "id": id, "user_id": userId, "active": false},
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

// PruneExpired deletes rows that are no longer usable: revoked tokens, and
// tokens past their (optional) expiry. Unlike device codes and refresh tokens,
// PAT expiry was previously only enforced lazily at read time, so expired and
// revoked rows accumulated indefinitely.
func (r *personalAccessTokenRepository) PruneExpired(ex lit.Executor, now time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM personal_access_tokens WHERE revoked = :revoked OR (expires_at IS NOT NULL AND expires_at < :cutoff)",
		lit.P{"revoked": true, "cutoff": shared.FormatAuthTime(now)},
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

func (r *personalAccessTokenRepository) TouchLastUsed(ex lit.Executor, id string, usedAt time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE personal_access_tokens SET last_used_at = :used WHERE id = :id",
		lit.P{"used": shared.FormatAuthTime(usedAt), "id": id},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

var PersonalAccessTokenRepository = personalAccessTokenRepository{}
