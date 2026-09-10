// Package remote is the runner side of the runner protocol: a Claimer and
// Reporter that talk to the backend's /api/agent-runners routes with the
// shared runner secret, so the harness runs unchanged on a host that has
// no database and no provider registry.
package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/agentrunner"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/agents"
	"github.com/tracewayapp/traceway/backend/app/models"
)

const (
	NameHeader         = "X-Traceway-Runner-Name"
	VersionHeader      = "X-Traceway-Runner-Version"
	CapabilitiesHeader = "X-Traceway-Runner-Capabilities"
	ClaimHeader        = "X-Traceway-Claim"

	pollTimeout    = 40 * time.Second
	requestTimeout = 60 * time.Second
	blobTimeout    = 5 * time.Minute
	maxErrorBytes  = 2048
)

// Capabilities is what a runner tells the backend about itself on every
// request; it is stored on the runner's row for the fleet view.
type Capabilities struct {
	SchemaVersion int      `json:"schemaVersion"`
	Agents        []string `json:"agents"`
	Sandbox       string   `json:"sandbox"`
	Workers       int      `json:"workers"`
}

// Client implements agentrunner.Claimer and agentrunner.Reporter over HTTP.
type Client struct {
	BaseURL      string
	Secret       string
	Name         string
	Version      string
	Capabilities Capabilities

	poll    *http.Client
	http    *http.Client
	claims  *sync.Map
	claimID string
}

func New(baseURL, secret, name, version string, capabilities Capabilities) *Client {
	return &Client{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		Secret:       secret,
		Name:         name,
		Version:      version,
		Capabilities: capabilities,
		claims:       &sync.Map{},
		poll:         &http.Client{Timeout: pollTimeout},
		http:         &http.Client{Timeout: requestTimeout},
	}
}

// ErrUnauthorized means the runner secret was rejected; the runner stops
// rather than retrying with a bad credential.
var ErrUnauthorized = errors.New("the runner secret was rejected")

func (c *Client) Claim(ctx context.Context, limit int) ([]*models.AgentAttempt, error) {
	var response struct {
		Attempts []*models.AgentAttempt `json:"attempts"`
	}
	if err := c.call(ctx, c.poll, http.MethodPost, "/api/agent-runners/poll", map[string]int{"maxAttempts": limit}, &response); err != nil {
		return nil, err
	}
	for _, attempt := range response.Attempts {
		c.claims.Store(attempt.Id.String(), attempt.ClaimedBy)
	}
	return response.Attempts, nil
}

func (c *Client) ForAttempt(attempt *models.AgentAttempt) (agentrunner.Claimer, agentrunner.Reporter) {
	bound := *c
	bound.claimID = attempt.ClaimedBy
	return &bound, &bound
}

func (c *Client) Inputs(ctx context.Context, attempt *models.AgentAttempt) (*agentrunner.Inputs, error) {
	var inputs agentrunner.Inputs
	if err := c.call(ctx, c.http, http.MethodGet, c.attemptPath(attempt.Id, "context"), nil, &inputs); err != nil {
		return nil, err
	}
	if inputs.SchemaVersion > agentrunner.ProtocolVersion {
		return nil, fmt.Errorf("the backend speaks runner protocol %d; this runner knows %d", inputs.SchemaVersion, agentrunner.ProtocolVersion)
	}
	return &inputs, nil
}

func (c *Client) GitCredential(ctx context.Context, attempt *models.AgentAttempt) (agent.GitCredential, error) {
	var cred struct {
		Username  string    `json:"username"`
		Password  string    `json:"password"`
		ExpiresAt time.Time `json:"expiresAt"`
	}
	if err := c.call(ctx, c.http, http.MethodPost, c.attemptPath(attempt.Id, "git-credential"), nil, &cred); err != nil {
		return agent.GitCredential{}, err
	}
	return agent.GitCredential{Username: cred.Username, Password: cred.Password, ExpiresAt: cred.ExpiresAt}, nil
}

func (c *Client) Transition(ctx context.Context, attemptId uuid.UUID, to string) error {
	return c.call(ctx, c.http, http.MethodPost, c.attemptPath(attemptId, "transition"), map[string]string{"to": to}, nil)
}

func (c *Client) RenewRunToken(ctx context.Context, id uuid.UUID) (*agent.RunToken, error) {
	var token agent.RunToken
	err := c.call(ctx, c.http, http.MethodPost, c.attemptPath(id, "run-token"), nil, &token)
	return &token, err
}

func (c *Client) Renew(ctx context.Context, attemptId uuid.UUID) (bool, error) {
	err := c.Events(ctx, attemptId, nil)
	if errors.Is(err, agentrunner.ErrClaimLost) {
		return false, nil
	}
	return err == nil, err
}

func (c *Client) Events(ctx context.Context, attemptId uuid.UUID, batch []agents.Event) error {
	if batch == nil {
		batch = []agents.Event{}
	}
	return c.call(ctx, c.http, http.MethodPost, c.attemptPath(attemptId, "events"), map[string]any{"schemaVersion": agentrunner.ProtocolVersion, "events": batch}, nil)
}

func (c *Client) Finding(ctx context.Context, attemptId uuid.UUID, body string) error {
	return c.call(ctx, c.http, http.MethodPost, c.attemptPath(attemptId, "findings"), map[string]string{"body": body}, nil)
}

func (c *Client) Blob(ctx context.Context, attemptId uuid.UUID, name string, content []byte, appendTo bool) error {
	path := c.attemptPath(attemptId, "blobs/"+url.PathEscape(name))
	if appendTo {
		path += "?append=1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.BaseURL+path, bytes.NewReader(content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	c.authorize(req)
	client := &http.Client{Timeout: blobTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.check(resp, nil)
}

func (c *Client) Finish(ctx context.Context, attemptId uuid.UUID, outcome agentrunner.Outcome) error {
	outcome.SchemaVersion = agentrunner.ProtocolVersion
	return c.call(ctx, c.http, http.MethodPost, c.attemptPath(attemptId, "result"), outcome, nil)
}

func (c *Client) attemptPath(attemptId uuid.UUID, tail string) string {
	return "/api/agent-runners/attempts/" + attemptId.String() + "/" + tail
}

func (c *Client) authorize(req *http.Request) {
	claimID := c.claimID
	if claimID == "" {
		parts := strings.Split(req.URL.Path, "/")
		if len(parts) > 4 && parts[3] == "attempts" {
			if value, ok := c.claims.Load(parts[4]); ok {
				claimID, _ = value.(string)
			}
		}
	}
	if claimID != "" {
		req.Header.Set(ClaimHeader, claimID)
	}
	req.Header.Set("Authorization", "Bearer "+c.Secret)
	req.Header.Set(NameHeader, c.Name)
	req.Header.Set(VersionHeader, c.Version)
	if encoded, err := json.Marshal(c.Capabilities); err == nil {
		req.Header.Set(CapabilitiesHeader, string(encoded))
	}
}

func (c *Client) call(ctx context.Context, client *http.Client, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.authorize(req)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.check(resp, out)
}

// check maps the protocol's statuses: 401 ends the runner, 409 is a lost
// claim, anything else non-2xx is an error carrying the body.
func (c *Client) check(resp *http.Response, out any) error {
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrUnauthorized
	case resp.StatusCode == http.StatusConflict:
		return agentrunner.ErrClaimLost
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		data, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
		return fmt.Errorf("%s %s: %d: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

var (
	_ agentrunner.Claimer  = (*Client)(nil)
	_ agentrunner.Reporter = (*Client)(nil)
)
