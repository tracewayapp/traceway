package github

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/secrets"
	"github.com/tracewayapp/traceway/backend/app/services"
)

const (
	manifestStateKind = "github-manifest"
	manifestStateTTL  = 15 * time.Minute
)

// ManifestStart is what the settings page receives after asking to start
// the flow: the page to open, which submits the manifest to GitHub.
type ManifestStart struct {
	URL string `json:"url"`
}

// StartManifest signs the state that ties GitHub's callback to the
// organization and integration row, and returns the page the browser opens.
func StartManifest(instanceURL string, organizationId int, integrationId int) (ManifestStart, error) {
	state, err := services.SignState(manifestStateKind, map[string]string{
		"organizationId": strconv.Itoa(organizationId),
		"integrationId":  strconv.Itoa(integrationId),
	}, manifestStateTTL)
	if err != nil {
		return ManifestStart{}, err
	}
	return ManifestStart{URL: strings.TrimRight(instanceURL, "/") + ManifestPath + "/start?state=" + state}, nil
}

// manifest is the GitHub App manifest: private, the inbound route as the
// webhook, the callback for the code exchange, and the permissions and
// events the agent uses.
func manifest(instanceURL string, integrationId int, state string) map[string]any {
	base := strings.TrimRight(instanceURL, "/")
	return map[string]any{
		"name":         "Traceway",
		"url":          base,
		"description":  "Fix issues from Traceway with a coding agent",
		"public":       false,
		"redirect_url": base + ManifestPath + "/callback",
		"hook_attributes": map[string]any{
			"url":    fmt.Sprintf("%s/api/integrations/%s/inbound/%d", base, Provider, integrationId),
			"active": true,
		},
		"default_permissions": installationPermissions,
		"default_events":      []string{"installation", "issues", "issue_comment", "pull_request", "pull_request_review_comment"},
	}
}

// ManifestPage renders the self-submitting form that sends the manifest to
// GitHub; the state travels in the form so the callback can verify it.
func ManifestPage(instanceURL string, state string) (string, error) {
	values, err := services.ParseState(manifestStateKind, state)
	if err != nil {
		return "", err
	}
	integrationId, _ := strconv.Atoi(values["integrationId"])
	encoded, err := json.Marshal(manifest(instanceURL, integrationId, state))
	if err != nil {
		return "", err
	}
	return `<!doctype html><html><head><meta charset="utf-8"><title>Create the Traceway GitHub App</title></head><body>
<p>Sending the App manifest to GitHub. If nothing happens, press the button.</p>
<form id="manifest" method="post" action="https://github.com/settings/apps/new?state=` + html.EscapeString(state) + `">
<input type="hidden" name="manifest" value="` + html.EscapeString(string(encoded)) + `">
<button type="submit">Create the GitHub App</button>
</form>
<script>document.getElementById("manifest").submit()</script>
</body></html>`, nil
}

type conversionResponse struct {
	Id            int64  `json:"id"`
	Slug          string `json:"slug"`
	HTMLURL       string `json:"html_url"`
	PEM           string `json:"pem"`
	WebhookSecret string `json:"webhook_secret"`
	ClientId      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
}

// CompleteManifest handles GitHub's callback: the state is verified, the
// temporary code is exchanged for the App's credentials, they are stored
// encrypted on the integration (created if the flow started without one),
// and the caller gets the install page to send the browser to.
func (h *Host) CompleteManifest(ctx context.Context, code string, state string, userId int) (string, error) {
	values, err := services.ParseState(manifestStateKind, state)
	if err != nil {
		return "", fmt.Errorf("the manifest state is invalid or expired: %w", err)
	}
	organizationId, _ := strconv.Atoi(values["organizationId"])
	integrationId, _ := strconv.Atoi(values["integrationId"])
	if organizationId == 0 || code == "" {
		return "", errors.New("the manifest callback is missing its code or organization")
	}
	var converted conversionResponse
	if err := h.call(ctx, "", http.MethodPost, "/app-manifests/"+code+"/conversions", nil, &converted); err != nil {
		return "", err
	}
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		now := time.Now().UTC()
		in, err := h.manifestTarget(tx, organizationId, integrationId, converted.Slug, userId, now)
		if err != nil {
			return struct{}{}, err
		}
		var cfg map[string]any
		if len(in.Config) > 0 {
			if err := json.Unmarshal([]byte(in.Config), &cfg); err != nil {
				return struct{}{}, err
			}
		}
		if cfg == nil {
			cfg = map[string]any{}
		}
		cfg["mode"] = ModeApp
		cfg["appId"] = strconv.FormatInt(converted.Id, 10)
		cfg["clientId"] = converted.ClientId
		cfg["clientSecret"] = converted.ClientSecret
		cfg["privateKey"] = converted.PEM
		cfg["webhookSecret"] = converted.WebhookSecret
		cfg["installationId"] = ""
		delete(cfg, "token")
		plain, err := json.Marshal(cfg)
		if err != nil {
			return struct{}{}, err
		}
		encrypted, err := secrets.EncryptFields(plain, secretFields)
		if err != nil {
			return struct{}{}, err
		}
		in.Config = models.JSONText(encrypted)
		in.UpdatedAt = now
		if in.Id == 0 {
			id, err := transactional.IntegrationRepository.Create(tx, in)
			if err != nil {
				return struct{}{}, err
			}
			in.Id = id
			return struct{}{}, nil
		}
		return struct{}{}, transactional.IntegrationRepository.Update(tx, in)
	})
	if err != nil {
		return "", err
	}
	agent.IntegrationsChanged()
	installURL := strings.TrimRight(h.WebBase, "/") + "/apps/" + converted.Slug + "/installations/new"
	if converted.HTMLURL != "" {
		installURL = strings.TrimRight(converted.HTMLURL, "/") + "/installations/new"
	}
	return installURL, nil
}

// manifestTarget is the integration row the credentials land on: the one
// the flow started from, or a new github row of the organization named
// after the App.
func (h *Host) manifestTarget(tx *sql.Tx, organizationId int, integrationId int, slug string, userId int, now time.Time) (*models.Integration, error) {
	if integrationId != 0 {
		in, err := transactional.IntegrationRepository.FindById(tx, integrationId)
		if err != nil {
			return nil, err
		}
		if in == nil || in.OrganizationId != organizationId || in.Provider != Provider {
			return nil, errors.New("the integration the manifest flow started from is gone")
		}
		return in, nil
	}
	name := "GitHub App"
	if slug != "" {
		name += " " + slug
	}
	in := &models.Integration{OrganizationId: organizationId, Provider: Provider, Kinds: models.StringSlice(h.Kinds()), Name: name, Config: models.JSONText(`{}`), Enabled: true, CreatedAt: now, UpdatedAt: now}
	if userId != 0 {
		in.CreatedBy = &userId
	}
	return in, nil
}
