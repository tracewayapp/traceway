package repositories

import (
	"backend/app/chdb"
	"backend/app/models"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
)

var ErrProjectNotFound = errors.New("project not found")

type projectRepository struct{}

func (p *projectRepository) FindAll(ctx context.Context) ([]models.Project, error) {
	rows, err := (*chdb.Conn).Query(ctx, "SELECT id, name, token, framework, created_at FROM projects ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var proj models.Project
		if err := rows.Scan(&proj.Id, &proj.Name, &proj.Token, &proj.Framework, &proj.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, proj)
	}
	return projects, nil
}

func (p *projectRepository) FindByToken(ctx context.Context, token string) (*models.Project, error) {
	var proj models.Project
	err := (*chdb.Conn).QueryRow(ctx, "SELECT id, name, token, framework, created_at FROM projects WHERE token = ?", token).
		Scan(&proj.Id, &proj.Name, &proj.Token, &proj.Framework, &proj.CreatedAt)
	if err != nil {
		return nil, ErrProjectNotFound
	}
	return &proj, nil
}

func (p *projectRepository) FindById(ctx context.Context, id string) (*models.Project, error) {
	var proj models.Project
	err := (*chdb.Conn).QueryRow(ctx, "SELECT id, name, token, framework, created_at FROM projects WHERE id = ?", id).
		Scan(&proj.Id, &proj.Name, &proj.Token, &proj.Framework, &proj.CreatedAt)
	if err != nil {
		return nil, ErrProjectNotFound
	}
	return &proj, nil
}

func (p *projectRepository) Create(ctx context.Context, name string, framework string) (*models.Project, error) {
	id := uuid.New().String()
	token := generateSecureToken()

	err := (*chdb.Conn).Exec(ctx, "INSERT INTO projects (id, name, token, framework) VALUES (?, ?, ?, ?)", id, name, token, framework)
	if err != nil {
		return nil, err
	}

	return p.FindById(ctx, id)
}

func generateSecureToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

var ProjectRepository = projectRepository{}
