package repositories

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type oauthClientRepository struct{}

// Create stores a dynamically registered OAuth client. The redirect URIs are
// serialized as a JSON array: they are only ever read back as a whole set for
// exact-match validation, never queried individually.
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
			"created_at":    formatAuthTime(client.CreatedAt),
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
	if c.CreatedAt, err = parseAuthTime(createdAt); err != nil {
		return nil, err
	}
	return &c, nil
}

var OauthClientRepository = oauthClientRepository{}
