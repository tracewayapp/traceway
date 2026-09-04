package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	traceway "go.tracewayapp.com"
)

const (
	githubAPIBase = "https://api.github.com"
	// githubMaxResponseBytes caps what is read back from an API response; only
	// the created issue's number is ever needed from it.
	githubMaxResponseBytes = 1 << 20
)

type GitHubAdapter struct {
	Token  string   `json:"token"`
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	Labels []string `json:"labels,omitempty"`

	// baseURL replaces api.github.com in tests. Unexported, so a channel
	// config can never redirect deliveries somewhere else.
	baseURL string
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

// Send opens an issue for the message. Deliveries that close a previously
// opened issue go through AdapterSend, which routes them to CloseIssue.
func (a *GitHubAdapter) Send(ctx context.Context, msg Message) error {
	_, err := a.CreateIssue(ctx, msg)
	return err
}

// CreateIssue opens the issue and returns its number, so the caller can record
// it against the exception it tracks. A number of 0 means the issue was
// created but its number could not be read back.
func (a *GitHubAdapter) CreateIssue(ctx context.Context, msg Message) (int, error) {
	payload := map[string]interface{}{
		"title": msg.Subject,
		"body":  msg.Body,
	}
	if len(a.Labels) > 0 {
		payload["labels"] = a.Labels
	}

	resp, err := a.do(ctx, http.MethodPost, a.issuesURL(a.Owner, a.Repo), payload)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		io.Copy(io.Discard, io.LimitReader(resp.Body, githubMaxResponseBytes))
		return 0, fmt.Errorf("GitHub returned status %d", resp.StatusCode)
	}

	var created struct {
		Number int `json:"number"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, githubMaxResponseBytes)).Decode(&created); err != nil {
		// The issue exists. Failing the delivery over a body we cannot read
		// would retry the create and open a duplicate, so report success and
		// let the caller skip recording it.
		return 0, nil
	}
	return created.Number, nil
}

// CloseIssue closes an issue this channel opened and comments why. An issue
// that is gone (deleted, or in a repository the token no longer reaches)
// counts as closed: retrying cannot bring it back.
func (a *GitHubAdapter) CloseIssue(ctx context.Context, owner, repo string, number int, comment string) error {
	target := fmt.Sprintf("%s/%d", a.issuesURL(owner, repo), number)
	resp, err := a.do(ctx, http.MethodPatch, target, map[string]interface{}{
		"state":        "closed",
		"state_reason": "completed",
	})
	if err != nil {
		return err
	}
	drainResponse(resp)

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub returned status %d", resp.StatusCode)
	}

	// The close is what the delivery promised; the comment is commentary. It is
	// posted after the close so a retry never doubles it up, and its failure is
	// reported rather than returned so the retry does not either.
	if comment != "" {
		if err := a.comment(ctx, target, comment); err != nil {
			traceway.CaptureException(fmt.Errorf("closed GitHub issue %s/%s#%d but failed to comment: %w", owner, repo, number, err))
		}
	}
	return nil
}

func (a *GitHubAdapter) comment(ctx context.Context, issueURL, body string) error {
	resp, err := a.do(ctx, http.MethodPost, issueURL+"/comments", map[string]interface{}{"body": body})
	if err != nil {
		return err
	}
	drainResponse(resp)
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("GitHub returned status %d", resp.StatusCode)
	}
	return nil
}

func (a *GitHubAdapter) issuesURL(owner, repo string) string {
	base := a.baseURL
	if base == "" {
		base = githubAPIBase
	}
	return fmt.Sprintf("%s/repos/%s/%s/issues", base, url.PathEscape(owner), url.PathEscape(repo))
}

func (a *GitHubAdapter) do(ctx context.Context, method, target string, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GitHub payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub request failed: %w", err)
	}
	return resp, nil
}

func drainResponse(resp *http.Response) {
	io.Copy(io.Discard, io.LimitReader(resp.Body, githubMaxResponseBytes))
	resp.Body.Close()
}
