//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type oauthClientRepository struct{}

func (r *oauthClientRepository) Create(ex lit.Executor, client *models.OauthClient) error {
	uris, err := json.Marshal(client.RedirectUris)
	if err != nil {
		return err
	}
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO oauth_clients (id, name, redirect_uris, created_at)
			VALUES (:id, :name, :redirect_uris, :created_at)`,
		lit.P{
			"id":            client.Id,
			"name":          client.Name,
			"redirect_uris": string(uris),
			"created_at":    shared.FormatAuthTime(client.CreatedAt),
		},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

func (r *oauthClientRepository) FindById(ex lit.Executor, id string) (*models.OauthClient, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"SELECT id, name, redirect_uris, created_at FROM oauth_clients WHERE id = :id",
		lit.P{"id": id},
	)
	if err != nil {
		return nil, err
	}

	var (
		c         models.OauthClient
		uris      string
		createdAt string
	)
	err = ex.QueryRow(query, args...).Scan(&c.Id, &c.Name, &uris, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(uris), &c.RedirectUris); err != nil {
		return nil, err
	}
	if c.CreatedAt, err = shared.ParseAuthTime(createdAt); err != nil {
		return nil, err
	}
	return &c, nil
}

const staleOauthClientRetention = 30 * 24 * time.Hour

func (r *oauthClientRepository) PruneStale(ex lit.Executor, now time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`DELETE FROM oauth_clients WHERE created_at < :cutoff
			AND NOT EXISTS (SELECT 1 FROM refresh_tokens WHERE refresh_tokens.client_id = oauth_clients.id)
			AND NOT EXISTS (SELECT 1 FROM authorization_codes WHERE authorization_codes.client_id = oauth_clients.id)`,
		lit.P{"cutoff": shared.FormatAuthTime(now.Add(-staleOauthClientRetention))},
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

var OauthClientRepository = oauthClientRepository{}
