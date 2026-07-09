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

type authorizationCodeRepository struct{}

func (r *authorizationCodeRepository) Create(ex lit.Executor, code, clientId string, userId int, redirectUri, codeChallenge string, expiresAt time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO authorization_codes (code_hash, client_id, user_id, redirect_uri, code_challenge, expires_at, created_at)
			VALUES (:hash, :client_id, :user_id, :redirect_uri, :code_challenge, :expires_at, :created_at)`,
		lit.P{
			"hash":           shared.HashAuthToken(code),
			"client_id":      clientId,
			"user_id":        userId,
			"redirect_uri":   redirectUri,
			"code_challenge": codeChallenge,
			"expires_at":     shared.FormatAuthTime(expiresAt),
			"created_at":     shared.FormatAuthTime(time.Now()),
		},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

func (r *authorizationCodeRepository) FindByCode(ex lit.Executor, code string) (*models.AuthorizationCode, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"SELECT code_hash, client_id, user_id, redirect_uri, code_challenge, expires_at, created_at FROM authorization_codes WHERE code_hash = :hash",
		lit.P{"hash": shared.HashAuthToken(code)},
	)
	if err != nil {
		return nil, err
	}

	var (
		c         models.AuthorizationCode
		expiresAt string
		createdAt string
	)
	err = ex.QueryRow(query, args...).Scan(&c.CodeHash, &c.ClientId, &c.UserId, &c.RedirectUri, &c.CodeChallenge, &expiresAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if c.ExpiresAt, err = shared.ParseAuthTime(expiresAt); err != nil {
		return nil, err
	}
	if c.CreatedAt, err = shared.ParseAuthTime(createdAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *authorizationCodeRepository) Delete(ex lit.Executor, code string) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM authorization_codes WHERE code_hash = :hash",
		lit.P{"hash": shared.HashAuthToken(code)},
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

func (r *authorizationCodeRepository) PruneExpired(ex lit.Executor, now time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM authorization_codes WHERE expires_at < :cutoff",
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

var AuthorizationCodeRepository = authorizationCodeRepository{}
