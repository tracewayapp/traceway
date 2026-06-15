package cmd

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/services"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

func seed(opts *options) error {
	if opts.defaultUser == nil {
		return nil
	}

	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		existing, err := repositories.UserRepository.FindByEmail(tx, opts.defaultUser.email)
		if err != nil {
			return struct{}{}, err
		}
		if existing != nil {
			config.Logf("Seed: user %s already exists, skipping", opts.defaultUser.email)
			return struct{}{}, nil
		}

		hash, err := services.HashPassword(opts.defaultUser.password)
		if err != nil {
			return struct{}{}, err
		}

		user, err := repositories.UserRepository.Create(tx, opts.defaultUser.email, "Admin", hash)
		if err != nil {
			return struct{}{}, err
		}

		org, err := repositories.OrganizationRepository.Create(tx, "Default", "UTC")
		if err != nil {
			return struct{}{}, err
		}

		_, err = repositories.OrganizationRepository.AddUser(tx, org.Id, user.Id, "owner")
		if err != nil {
			return struct{}{}, err
		}

		for _, p := range opts.defaultProjects {
			project := &models.Project{
				Id:             uuid.New(),
				Name:           p.name,
				Token:          p.token,
				Framework:      p.framework,
				OrganizationId: &org.Id,
				CreatedAt:      time.Now().UTC(),
			}
			if p.sourceMapToken != "" {
				token := p.sourceMapToken
				project.SourceMapToken = &token
			}
			if err := lit.InsertExistingUuid(tx, project); err != nil {
				return struct{}{}, err
			}
			config.Logf("Seed: project %q connection string: %s@%s/api/report", project.Name, project.Token, opts.serverURL)
		}

		config.Logf("Seed: created user %s, org %q", user.Email, org.Name)
		return struct{}{}, nil
	})

	return err
}
