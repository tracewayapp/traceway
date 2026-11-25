package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/outbox"
)

// GitHub rejects a longer title or body with 422.
const (
	githubMaxTitleChars = 256
	githubMaxBodyChars  = 65536
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

	issueBody := msg.Body
	if isAbsoluteURL(msg.URL) && !strings.Contains(issueBody, msg.URL) {
		issueBody += "\n\n[View in Traceway](" + msg.URL + ")"
	}

	payload := map[string]interface{}{
		"title": truncateRunes(msg.Subject, githubMaxTitleChars),
		"body":  truncateRunes(issueBody, githubMaxBodyChars),
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
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != 201 {
		err := fmt.Errorf("GitHub returned status %d%s", resp.StatusCode, githubErrorDetail(detail))
		switch resp.StatusCode {
		// The delivery carries a snapshot of the channel, so a rejected token,
		// a missing repository or an invalid issue fails the same way on every
		// retry. 403 and 429 stay retryable: they are how GitHub rate limits.
		case http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusGone, http.StatusUnprocessableEntity:
			return &outbox.PermanentError{Err: err}
		}
		return err
	}

	return nil
}

// githubErrorDetail renders the reason GitHub gives for a refusal, e.g.
// ": Validation Failed (title too_long)", so a failed delivery says why.
func githubErrorDetail(body []byte) string {
	var parsed struct {
		Message string `json:"message"`
		Errors  []struct {
			Field   string `json:"field"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &parsed) != nil || parsed.Message == "" {
		return ""
	}
	detail := ": " + parsed.Message
	var reasons []string
	for _, e := range parsed.Errors {
		reason := strings.TrimSpace(e.Field + " " + e.Code + " " + e.Message)
		if reason != "" {
			reasons = append(reasons, reason)
		}
	}
	if len(reasons) > 0 {
		detail += " (" + strings.Join(reasons, "; ") + ")"
	}
	return truncateRunes(detail, 500)
}
