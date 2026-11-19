package env

import (
	"context"
	"fmt"
	"net/http"
)

// Project is one Traceway project of the instance.
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Framework string `json:"framework"`
	Token     string `json:"-"` // ingest token; never written to seed.json
}

// Admin is the registered owner of the instance's one organization.
type Admin struct {
	Email    string
	Password string
	UserID   int
	OrgID    int
	JWT      string
}

// Register creates the instance's owner and organization. A self-hosted
// instance allows exactly one registration, which is why every run boots its
// own backend.
func Register(ctx context.Context, b *Backend, email, password, org string) (*Admin, error) {
	res, err := b.API().MustOK(ctx, http.MethodPost, "/api/register", nil, map[string]any{
		"email": email, "name": "Eval Admin", "password": password,
		"organizationName": org, "timezone": "UTC",
	})
	if err != nil {
		return nil, err
	}
	var out struct {
		Token string `json:"token"`
		User  struct {
			ID int `json:"id"`
		} `json:"user"`
		Organizations []struct {
			ID int `json:"id"`
		} `json:"organizations"`
	}
	if err := res.JSON(&out); err != nil {
		return nil, err
	}
	if out.Token == "" || len(out.Organizations) == 0 {
		return nil, fmt.Errorf("register returned no token or organization: %s", clip(res.Body))
	}
	return &Admin{Email: email, Password: password, UserID: out.User.ID, OrgID: out.Organizations[0].ID, JWT: out.Token}, nil
}

// CreateProjects creates projects in the admin's organization in one batch
// and returns them with their ingest tokens.
func CreateProjects(ctx context.Context, b *Backend, a *Admin, specs []Project) ([]Project, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	body := map[string]any{"organizationId": a.OrgID}
	var list []map[string]string
	for _, p := range specs {
		list = append(list, map[string]string{"name": p.Name, "framework": p.Framework})
	}
	body["projects"] = list
	res, err := b.API().WithBearer(a.JWT).MustOK(ctx, http.MethodPost, "/api/projects/batch", nil, body)
	if err != nil {
		return nil, err
	}
	var out struct {
		Projects []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Framework string `json:"framework"`
			Token     string `json:"token"`
		} `json:"projects"`
	}
	if err := res.JSON(&out); err != nil {
		return nil, err
	}
	if len(out.Projects) != len(specs) {
		return nil, fmt.Errorf("batch create returned %d projects, want %d", len(out.Projects), len(specs))
	}
	projects := make([]Project, 0, len(out.Projects))
	for _, p := range out.Projects {
		projects = append(projects, Project{ID: p.ID, Name: p.Name, Framework: p.Framework, Token: p.Token})
	}
	return projects, nil
}

// CreatePAT mints a personal access token for the bearer's user. Write
// tokens must expire; expiresInDays 0 means "never" for read tokens.
func CreatePAT(ctx context.Context, b *Backend, jwt, name, scope string, expiresInDays int) (string, error) {
	body := map[string]any{"name": name, "scope": scope}
	if expiresInDays > 0 {
		body["expiresInDays"] = expiresInDays
	}
	res, err := b.API().WithBearer(jwt).MustOK(ctx, http.MethodPost, "/api/personal-access-tokens", nil, body)
	if err != nil {
		return "", err
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := res.JSON(&out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", fmt.Errorf("PAT create returned no token")
	}
	return out.Token, nil
}
