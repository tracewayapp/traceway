package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type GitHubAdapter struct {
	Token  string   `json:"token"`
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	Labels []string `json:"labels,omitempty"`
}

func (a *GitHubAdapter) Type() string { return "github" }

func (a *GitHubAdapter) Validate() error {
	if a.Token == "" {
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

func (a *GitHubAdapter) Send(ctx context.Context, msg Message) error {
	// every send opens an issue, and an open issue saying the problem is over
	// is noise nobody closes
	if msg.Recovered {
		return nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", a.Owner, a.Repo)

	payload := map[string]interface{}{
		"title": msg.Subject,
		"body":  msg.Body,
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
	req.Header.Set("Authorization", "Bearer "+a.Token)
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
