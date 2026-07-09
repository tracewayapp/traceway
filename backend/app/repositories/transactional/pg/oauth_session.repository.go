//go:build transactional_pg

package pg

import (
	"database/sql"
	"errors"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
)

type oauthSessionRepository struct{}

func (r *oauthSessionRepository) Get(tx *sql.Tx, id string) ([]byte, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"SELECT data FROM oauth_sessions WHERE id = :id AND expires_at > :now",
		lit.P{"id": id, "now": time.Now().UTC().Format(time.RFC3339Nano)},
	)
	if err != nil {
		return nil, err
	}

	var data []byte
	if err := tx.QueryRow(query, args...).Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func (r *oauthSessionRepository) Save(tx *sql.Tx, id string, data []byte, expiresAt time.Time) error {
	deleteQuery, deleteArgs, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM oauth_sessions WHERE id = :id",
		lit.P{"id": id},
	)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(deleteQuery, deleteArgs...); err != nil {
		return err
	}

	insertQuery, insertArgs, err := lit.ParseNamedQuery(
		db.Driver,
		"INSERT INTO oauth_sessions (id, data, expires_at) VALUES (:id, :data, :expires_at)",
		lit.P{
			"id":         id,
			"data":       data,
			"expires_at": expiresAt.UTC().Format(time.RFC3339Nano),
		},
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(insertQuery, insertArgs...)
	return err
}

func (r *oauthSessionRepository) Delete(tx *sql.Tx, id string) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM oauth_sessions WHERE id = :id",
		lit.P{"id": id},
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(query, args...)
	return err
}

func (r *oauthSessionRepository) PruneExpired(tx *sql.Tx, now time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM oauth_sessions WHERE expires_at < :cutoff",
		lit.P{"cutoff": now.UTC().Format(time.RFC3339Nano)},
	)
	if err != nil {
		return 0, err
	}
	result, err := tx.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

var OAuthSessionRepository = oauthSessionRepository{}
