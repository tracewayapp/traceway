//go:build transactional_pg

package pg

import (
	"database/sql"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type refreshTokenRepository struct{}

func (r *refreshTokenRepository) Insert(ex lit.Executor, token, familyId string, userId int, clientId string, expiresAt time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO refresh_tokens (token_hash, family_id, user_id, client_id, revoked, used, expires_at, created_at)
			VALUES (:hash, :family, :user_id, :client_id, :revoked, :used, :expires_at, :created_at)`,
		lit.P{
			"hash":       shared.HashAuthToken(token),
			"family":     familyId,
			"user_id":    userId,
			"client_id":  clientId,
			"revoked":    false,
			"used":       false,
			"expires_at": shared.FormatAuthTime(expiresAt),
			"created_at": shared.FormatAuthTime(time.Now()),
		},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

func (r *refreshTokenRepository) FindByToken(ex lit.Executor, token string) (*models.RefreshToken, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"SELECT token_hash, family_id, user_id, client_id, revoked, used, used_at, expires_at, created_at FROM refresh_tokens WHERE token_hash = :hash",
		lit.P{"hash": shared.HashAuthToken(token)},
	)
	if err != nil {
		return nil, err
	}

	var (
		t         models.RefreshToken
		usedAt    sql.NullString
		expiresAt string
		createdAt string
	)
	err = ex.QueryRow(query, args...).Scan(&t.TokenHash, &t.FamilyId, &t.UserId, &t.ClientId, &t.Revoked, &t.Used, &usedAt, &expiresAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if t.UsedAt, err = shared.ParseNullableAuthTime(usedAt); err != nil {
		return nil, err
	}
	if t.ExpiresAt, err = shared.ParseAuthTime(expiresAt); err != nil {
		return nil, err
	}
	if t.CreatedAt, err = shared.ParseAuthTime(createdAt); err != nil {
		return nil, err
	}
	return &t, nil
}

// MarkUsedIfUnused atomically flips the token to used, but only if it is still
// unused and not revoked. The rows-affected result is the concurrency guard: a
// return of 0 means another request already consumed (or revoked) this token,
// which is how RotateRefresh distinguishes a benign concurrent retry from a
// genuine replay of an already-rotated token.
func (r *refreshTokenRepository) MarkUsedIfUnused(ex lit.Executor, token string, usedAt time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE refresh_tokens SET used = :used, used_at = :used_at WHERE token_hash = :hash AND used = :active AND revoked = :active",
		lit.P{"used": true, "used_at": shared.FormatAuthTime(usedAt), "hash": shared.HashAuthToken(token), "active": false},
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

func (r *refreshTokenRepository) RevokeFamily(ex lit.Executor, familyId string) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE refresh_tokens SET revoked = :revoked WHERE family_id = :family",
		lit.P{"revoked": true, "family": familyId},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

// usedRefreshTokenRetention is how long a consumed (rotated) token row is kept
// so a genuine replay can still be detected and revoke its family. Without a
// cap, every rotation would leave a row behind for the full 90-day expiry;
// after a month the row's only value is replay detection for a token an
// attacker can no longer redeem, so it is safe to drop. Revoked rows carry no
// signal at all (the family is already dead) and are dropped immediately,
// matching the PAT prune.
const usedRefreshTokenRetention = 30 * 24 * time.Hour

func (r *refreshTokenRepository) PruneExpired(ex lit.Executor, now time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM refresh_tokens WHERE expires_at < :cutoff OR revoked = :revoked OR (used = :used AND used_at < :used_cutoff)",
		lit.P{
			"cutoff":      shared.FormatAuthTime(now),
			"revoked":     true,
			"used":        true,
			"used_cutoff": shared.FormatAuthTime(now.Add(-usedRefreshTokenRetention)),
		},
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

var RefreshTokenRepository = refreshTokenRepository{}
