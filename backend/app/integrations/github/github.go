// Package github is GitHub as a code host, issue tracker and chat surface
// for the fix agent. One integration row is one GitHub connection with one
// of two credential modes: a fine-grained personal access token, or a
// GitHub App created from a manifest whose installation tokens are minted
// per repository. Raw HTTP is the house style; there is no go-github
// dependency.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/secrets"
)

const (
	Provider       = "github"
	defaultAPIBase = "https://api.github.com"
	defaultWebBase = "https://github.com"
	defaultHost    = "github.com"
	requestTimeout = 30 * time.Second
	maxErrorBytes  = 4096

	ModePAT = "pat"
	ModeApp = "app"

	// ManifestPath is the API route the settings page POSTs to start the
	// manifest flow; the provider's SetupFlow names it.
	ManifestPath = "/api/integrations/github/manifest"

	defaultLabel = "traceway"
)

var secretFields = []string{"token", "privateKey", "webhookSecret", "clientSecret"}

var labelRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._:/-]{0,49}$`)

// Host is the provider over one GitHub deployment. APIBase, WebBase and
// CloneHost default to github.com; tests point them at a local server.
type Host struct {
	APIBase   string
	WebBase   string
	CloneHost string
	Client    *http.Client
	tokens    *tokenCache
}

func New() *Host {
	return &Host{APIBase: defaultAPIBase, WebBase: defaultWebBase, CloneHost: defaultHost, Client: &http.Client{Timeout: requestTimeout}, tokens: newTokenCache()}
}

// Register wires GitHub into the agent registries and lets the github
// notification channel mint tokens from an integration.
func Register(h *Host) {
	agent.RegisterCodeHost(h)
	agent.RegisterIssueTracker(h)
	agent.RegisterChannel(channelView{h})
	agent.RegisterTrigger(triggerView{h})
	notifications.RegisterGitHubTokenSource(h.TokenForIntegration)
}

func (h *Host) Provider() string { return Provider }
func (h *Host) Kinds() []string {
	return []string{agent.KindCodeHost, agent.KindIssueTracker, agent.KindChat, agent.KindTrigger}
}

func (h *Host) Fields() []agent.Field {
	return []agent.Field{
		{Key: "mode", Label: "Credential", Kind: agent.FieldSelect, Options: []string{ModePAT, ModeApp}, Required: true,
			Help: "pat: a fine-grained personal access token. app: a GitHub App created from the manifest below, with installation tokens per repository and webhooks for comments and merges."},
		{Key: "token", Label: "Personal access token", Kind: agent.FieldSecret,
			Help: "pat mode. A fine-grained token with contents, pull requests and issues read and write on the repositories the agent may touch."},
		{Key: "label", Label: "Issue label", Kind: agent.FieldText,
			Help: "app mode. Labelling a GitHub issue that carries a Traceway hash with this label starts an attempt (default traceway)."},
		{Key: "appId", Label: "App ID", Kind: agent.FieldText, Help: "app mode. Filled by the manifest flow."},
		{Key: "clientId", Label: "Client ID", Kind: agent.FieldText, Help: "app mode. Filled by the manifest flow."},
		{Key: "clientSecret", Label: "Client secret", Kind: agent.FieldSecret, Help: "app mode. Filled by the manifest flow."},
		{Key: "privateKey", Label: "Private key", Kind: agent.FieldSecret, Help: "app mode. The App's PEM private key; filled by the manifest flow."},
		{Key: "webhookSecret", Label: "Webhook secret", Kind: agent.FieldSecret, Help: "app mode. Filled by the manifest flow."},
		{Key: "installationId", Label: "Installation ID", Kind: agent.FieldText, Help: "app mode. Recorded when the App is installed; the install page fills it."},
	}
}

// SetupFlow names the API route the settings page POSTs
// {organizationId, integrationId} to; the answer is the page to open.
func (h *Host) SetupFlow() *agent.SetupFlow {
	return &agent.SetupFlow{Label: "Create the GitHub App from a manifest", URL: strings.TrimPrefix(ManifestPath, "/api")}
}

func (h *Host) Validate(cfg map[string]string) error {
	mode := strings.TrimSpace(cfg["mode"])
	if mode == "" {
		mode = ModePAT
	}
	switch mode {
	case ModePAT:
		if strings.TrimSpace(cfg["token"]) == "" {
			return errors.New("A personal access token is required.")
		}
	case ModeApp:
		if strings.TrimSpace(cfg["appId"]) != "" {
			if _, err := strconv.ParseInt(strings.TrimSpace(cfg["appId"]), 10, 64); err != nil {
				return errors.New("The App ID must be a number.")
			}
			if strings.TrimSpace(cfg["privateKey"]) == "" || strings.TrimSpace(cfg["webhookSecret"]) == "" {
				return errors.New("An App needs its private key and webhook secret; run the manifest flow to fill them.")
			}
		}
		if id := strings.TrimSpace(cfg["installationId"]); id != "" {
			if _, err := strconv.ParseInt(id, 10, 64); err != nil {
				return errors.New("The installation ID must be a number.")
			}
		}
	default:
		return errors.New("The credential must be pat or app.")
	}
	if label := strings.TrimSpace(cfg["label"]); label != "" && !labelRe.MatchString(label) {
		return errors.New("The issue label may only contain letters, digits, spaces and . _ : / -.")
	}
	return nil
}

// settings is an integration's decrypted config.
type settings struct {
	Mode           string `json:"mode"`
	Token          string `json:"token"`
	Label          string `json:"label"`
	AppId          string `json:"appId"`
	ClientId       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	PrivateKey     string `json:"privateKey"`
	WebhookSecret  string `json:"webhookSecret"`
	InstallationId string `json:"installationId"`
}

func (s settings) mode() string {
	if s.Mode == "" {
		return ModePAT
	}
	return s.Mode
}

func (s settings) label() string {
	if strings.TrimSpace(s.Label) == "" {
		return defaultLabel
	}
	return strings.TrimSpace(s.Label)
}

func (s settings) installed() bool {
	return s.mode() == ModeApp && s.AppId != "" && s.InstallationId != ""
}

func (h *Host) settings(in *models.Integration) (settings, error) {
	decrypted, err := secrets.DecryptFields(json.RawMessage(in.Config), secretFields)
	if err != nil {
		return settings{}, err
	}
	var s settings
	if err := json.Unmarshal(decrypted, &s); err != nil {
		return settings{}, err
	}
	return s, nil
}

// Describe is the one-line state the repository tab shows next to the
// integration.
func (h *Host) Describe(in *models.Integration) string {
	s, err := h.settings(in)
	if err != nil {
		return "unreadable config"
	}
	switch {
	case s.mode() == ModePAT:
		return "personal access token"
	case s.AppId == "":
		return "GitHub App not created yet"
	case s.InstallationId == "":
		return "GitHub App created, not installed"
	}
	return "GitHub App installed"
}

func (h *Host) Ready(in *models.Integration) bool {
	s, err := h.settings(in)
	if err != nil {
		return false
	}
	switch s.mode() {
	case ModePAT:
		return strings.TrimSpace(s.Token) != ""
	case ModeApp:
		return s.installed() && strings.TrimSpace(s.PrivateKey) != ""
	default:
		return false
	}
}

// token is the bearer for API calls on a repository: the PAT, or an
// installation token minted (and cached) for the App.
func (h *Host) token(ctx context.Context, in *models.Integration, repo *models.Repository) (string, time.Time, error) {
	s, err := h.settings(in)
	if err != nil {
		return "", time.Time{}, err
	}
	if s.mode() == ModePAT {
		if s.Token == "" {
			return "", time.Time{}, errors.New("github integration has no token")
		}
		return s.Token, time.Time{}, nil
	}
	if !s.installed() {
		return "", time.Time{}, errors.New("the GitHub App is not installed yet; finish the manifest flow and install it on the repository")
	}
	return h.installationToken(ctx, in.Id, s, repo)
}

// TokenForIntegration is what the github notification channel uses to
// create issues through an integration instead of its own token.
func (h *Host) TokenForIntegration(ctx context.Context, in *models.Integration, owner string, name string) (string, error) {
	token, _, err := h.token(ctx, in, &models.Repository{Owner: owner, Name: name})
	return token, err
}

// CloneURL is the https remote of a repository.
func (h *Host) CloneURL(repo *models.Repository) string {
	return "https://" + h.CloneHost + "/" + repo.Owner + "/" + repo.Name + ".git"
}

// CloneCredential presents the token the way git over https expects. A PAT
// is as long-lived as the token, so ExpiresAt is left to the harness's own
// run bound; an installation token carries its hour.
func (h *Host) CloneCredential(ctx context.Context, in *models.Integration, repo *models.Repository) (agent.GitCredential, error) {
	token, expires, err := h.token(ctx, in, repo)
	if err != nil {
		return agent.GitCredential{}, err
	}
	return agent.GitCredential{Username: "x-access-token", Password: token, ExpiresAt: expires}, nil
}

type pullResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

func (h *Host) OpenPullRequest(ctx context.Context, in *models.Integration, repo *models.Repository, pr agent.PullRequest) (agent.Link, error) {
	token, _, err := h.token(ctx, in, repo)
	if err != nil {
		return agent.Link{}, err
	}
	var created pullResponse
	path := fmt.Sprintf("/repos/%s/%s/pulls", repo.Owner, repo.Name)
	findExisting := func() (bool, error) {
		var existing []pullResponse
		query := url.Values{"state": {"all"}, "head": {repo.Owner + ":" + pr.Head}, "base": {pr.Base}, "per_page": {"1"}}
		if err := h.call(ctx, token, http.MethodGet, path+"?"+query.Encode(), nil, &existing); err != nil {
			return false, err
		}
		if len(existing) == 0 {
			return false, nil
		}
		created = existing[0]
		return true, nil
	}
	found, err := findExisting()
	if err != nil {
		return agent.Link{}, err
	}
	body := map[string]any{"title": pr.Title, "body": pr.Body, "head": pr.Head, "base": pr.Base, "draft": pr.Draft}
	if !found {
		if err := h.call(ctx, token, http.MethodPost, path, body, &created); err != nil {
			// A lost response can follow a successful creation; reconcile before retrying.
			if found, _ := findExisting(); !found {
				return agent.Link{}, err
			}
		}
	}
	return agent.Link{
		Provider:      Provider,
		Kind:          models.LinkKindPR,
		ExternalRef:   fmt.Sprintf("%s/%s#%d", repo.Owner, repo.Name, created.Number),
		URL:           created.HTMLURL,
		IntegrationId: in.Id,
	}, nil
}

// Comment posts on the issue or pull request a link names, both of which
// GitHub addresses as issues.
func (h *Host) Comment(ctx context.Context, in *models.Integration, target agent.Link, body string) error {
	owner, name, number, err := splitRef(target.ExternalRef)
	if err != nil {
		return err
	}
	token, _, err := h.token(ctx, in, &models.Repository{Owner: owner, Name: name})
	if err != nil {
		return err
	}
	return h.call(ctx, token, http.MethodPost, fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, name, number), map[string]string{"body": body}, nil)
}

type issueResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

func (h *Host) OpenIssue(ctx context.Context, in *models.Integration, repo *models.Repository, issue agent.Issue) (agent.Link, error) {
	token, _, err := h.token(ctx, in, repo)
	if err != nil {
		return agent.Link{}, err
	}
	var created issueResponse
	if err := h.call(ctx, token, http.MethodPost, fmt.Sprintf("/repos/%s/%s/issues", repo.Owner, repo.Name), map[string]string{"title": issue.Title, "body": issue.Body}, &created); err != nil {
		return agent.Link{}, err
	}
	return agent.Link{
		Provider:      Provider,
		Kind:          models.LinkKindIssue,
		ExternalRef:   fmt.Sprintf("%s/%s#%d", repo.Owner, repo.Name, created.Number),
		URL:           created.HTMLURL,
		IntegrationId: in.Id,
	}, nil
}

func (h *Host) call(ctx context.Context, token, method, path string, body any, out any) error {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(h.APIBase, "/")+path, payload)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "traceway-agent")
	resp, err := h.Client.Do(req)
	if err != nil {
		return fmt.Errorf("github %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
		return fmt.Errorf("github %s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(detail)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// splitRef parses "owner/name#number".
func splitRef(ref string) (owner, name string, number int, err error) {
	repoPart, numberPart, ok := strings.Cut(ref, "#")
	if !ok {
		return "", "", 0, fmt.Errorf("github ref %q is not owner/name#number", ref)
	}
	owner, name, ok = strings.Cut(repoPart, "/")
	if !ok {
		return "", "", 0, fmt.Errorf("github ref %q is not owner/name#number", ref)
	}
	number, err = strconv.Atoi(numberPart)
	if err != nil {
		return "", "", 0, fmt.Errorf("github ref %q is not owner/name#number", ref)
	}
	return owner, name, number, nil
}

func issueRef(owner, name string, number int) string {
	return fmt.Sprintf("%s/%s#%d", owner, name, number)
}

var (
	_ agent.CodeHost      = (*Host)(nil)
	_ agent.IssueTracker  = (*Host)(nil)
	_ agent.Channel       = channelView{}
	_ agent.TriggerSource = triggerView{}
)
