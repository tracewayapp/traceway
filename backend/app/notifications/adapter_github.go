package notifications

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

type GitHubAdapter struct {
	Token  string   `json:"token"`
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	Labels []string `json:"labels,omitempty"`
	// Integration, when set, is the github integration whose credential
	// creates the issue instead of Token (an App's installation token or
	// the integration's own PAT).
	Integration int `json:"integrationId,omitempty"`
}

// GitHubTokenSource mints or returns the token an integration holds for a
// repository; the github provider registers it at boot.
type GitHubTokenSource func(ctx context.Context, in *models.Integration, owner string, name string) (string, error)

var githubTokenSource GitHubTokenSource

func RegisterGitHubTokenSource(source GitHubTokenSource) {
	githubTokenSource = source
}

func (a *GitHubAdapter) Type() string { return "github" }

func (a *GitHubAdapter) IntegrationId() int { return a.Integration }

func (a *GitHubAdapter) Validate() error {
	if a.Token == "" && a.Integration == 0 {
		return fmt.Errorf("GitHub token is required")
	}
	if a.Owner == "" {
		return fmt.Errorf("GitHub owner is required")
	}
	if a.Repo == "" {
		return fmt.Errorf("GitHub repo is required")
	}
	return nil
}

func (a *GitHubAdapter) token(ctx context.Context) (string, error) {
	if a.Integration == 0 {
		return a.Token, nil
	}
	if githubTokenSource == nil {
		return "", fmt.Errorf("github integrations are not registered")
	}
	in, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Integration, error) {
		return transactional.IntegrationRepository.FindById(tx, a.Integration)
	})
	if err != nil {
		return "", err
	}
	if in == nil || in.Provider != "github" || !in.Enabled {
		return "", fmt.Errorf("github integration %d is gone or disabled", a.Integration)
	}
	return githubTokenSource(ctx, in, a.Owner, a.Repo)
}

func (a *GitHubAdapter) Send(ctx context.Context, msg Message) error {
	token, err := a.token(ctx)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", a.Owner, a.Repo)

	payload := map[string]interface{}{
		"title": msg.Subject,
		"body":  issueBody(msg),
	}
	if len(a.Labels) > 0 {
		payload["labels"] = a.Labels
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal GitHub payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create GitHub request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != 201 {
		return fmt.Errorf("GitHub returned status %d", resp.StatusCode)
	}

	return nil
}

// issueBody adds the project line next to the Hash line the message
// already carries, so labelling the issue can start an attempt without
// trusting anything else in it.
func issueBody(msg Message) string {
	body := msg.Body
	if msg.ProjectId == "" || strings.Contains(body, "\nProject: ") || strings.HasPrefix(body, "Project: ") {
		return body
	}
	return strings.TrimRight(body, "\n") + "\nProject: " + msg.ProjectId + "\n"
}
