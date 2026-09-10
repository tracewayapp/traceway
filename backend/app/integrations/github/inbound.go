package github

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

const (
	maxInboundBytes = 1 << 20
	mentionToken    = "@traceway"
)

var (
	// hashLineRe is the prepare action's rule: the issue body names the
	// exception on a line of its own, and nothing else in it is trusted.
	hashLineRe    = regexp.MustCompile(`(?m)^Hash: ([0-9a-f]{16})$`)
	projectLineRe = regexp.MustCompile(`(?m)^Project: ([0-9a-fA-F-]{36})$`)
	hashRe        = regexp.MustCompile(`^[0-9a-f]{16}$`)
)

var errBadSignature = errors.New("github: webhook signature rejected")

// webhook is one delivery reduced to what the handlers need.
type webhook struct {
	event   string
	action  string
	owner   string
	repo    string
	number  int
	title   string
	body    string
	labels  []string
	label   string
	sender  string
	merged  bool
	isPR    bool
	comment struct {
		id   int64
		body string
		user string
	}
	installation struct {
		id int64
	}
}

type ghUser struct {
	Login string `json:"login"`
}

type ghRepo struct {
	Name  string `json:"name"`
	Owner ghUser `json:"owner"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghIssue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Labels      []ghLabel `json:"labels"`
	PullRequest *struct{} `json:"pull_request"`
}

type ghPayload struct {
	Action      string `json:"action"`
	Repository  ghRepo `json:"repository"`
	Sender      ghUser `json:"sender"`
	Label       *ghLabel
	Issue       *ghIssue `json:"issue"`
	PullRequest *struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Merged bool   `json:"merged"`
	} `json:"pull_request"`
	Comment *struct {
		Id   int64  `json:"id"`
		Body string `json:"body"`
		User ghUser `json:"user"`
	} `json:"comment"`
	Installation *struct {
		Id int64 `json:"id"`
	} `json:"installation"`
}

// parseRequest verifies X-Hub-Signature-256 with the integration's webhook
// secret and decodes the delivery. The body is put back so every port of
// the provider can read the same request.
func (h *Host) parseRequest(in *models.Integration, r *http.Request) (webhook, error) {
	s, err := h.settings(in)
	if err != nil {
		return webhook{}, err
	}
	if s.mode() != ModeApp || s.WebhookSecret == "" {
		return webhook{}, errors.New("github: this integration has no webhook secret; only App integrations receive webhooks")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxInboundBytes+1))
	if err != nil {
		return webhook{}, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) > maxInboundBytes {
		return webhook{}, errors.New("github: webhook body too large")
	}
	if !signatureMatches(r.Header.Get("X-Hub-Signature-256"), s.WebhookSecret, body) {
		return webhook{}, errBadSignature
	}
	return parseWebhook(r.Header.Get("X-GitHub-Event"), body)
}

func signatureMatches(header string, secret string, body []byte) bool {
	presented, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(presented))
}

// Sign renders the header a delivery of body would carry; tests and the
// docs example use it.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func parseWebhook(event string, body []byte) (webhook, error) {
	var payload ghPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return webhook{}, fmt.Errorf("github: decode %s payload: %w", event, err)
	}
	hook := webhook{event: event, action: payload.Action, owner: payload.Repository.Owner.Login, repo: payload.Repository.Name, sender: payload.Sender.Login}
	if payload.Installation != nil {
		hook.installation.id = payload.Installation.Id
	}
	if payload.Label != nil {
		hook.label = payload.Label.Name
	}
	if payload.Issue != nil {
		hook.number, hook.title, hook.body, hook.isPR = payload.Issue.Number, payload.Issue.Title, payload.Issue.Body, payload.Issue.PullRequest != nil
		for _, label := range payload.Issue.Labels {
			hook.labels = append(hook.labels, label.Name)
		}
	}
	if payload.PullRequest != nil {
		hook.number, hook.title, hook.merged, hook.isPR = payload.PullRequest.Number, payload.PullRequest.Title, payload.PullRequest.Merged, true
	}
	if payload.Comment != nil {
		hook.comment.id, hook.comment.body, hook.comment.user = payload.Comment.Id, payload.Comment.Body, payload.Comment.User.Login
	}
	return hook, nil
}

// Respond acknowledges GitHub's ping and records installations: both are
// transport-level and produce no attempt events.
func (h *Host) Respond(in *models.Integration, r *http.Request) ([]byte, bool, error) {
	hook, err := h.parseRequest(in, r)
	if err != nil {
		return nil, false, err
	}
	switch hook.event {
	case "ping":
		return []byte(`{"ok":true}`), true, nil
	case "installation":
		if err := h.recordInstallation(in, hook); err != nil {
			return nil, false, err
		}
		return []byte(`{"ok":true}`), true, nil
	}
	return nil, false, nil
}

// recordInstallation stores the installation id when the App is installed
// and clears it when it is removed, so the repository tab and the token
// path know where the App stands.
func (h *Host) recordInstallation(in *models.Integration, hook webhook) error {
	value := ""
	switch hook.action {
	case "created", "unsuspend", "new_permissions_accepted":
		if hook.installation.id == 0 {
			return nil
		}
		value = strconv.FormatInt(hook.installation.id, 10)
	case "deleted", "suspend":
	default:
		return nil
	}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		current, err := transactional.IntegrationRepository.FindById(tx, in.Id)
		if err != nil || current == nil {
			return struct{}{}, err
		}
		var cfg map[string]any
		if err := json.Unmarshal([]byte(current.Config), &cfg); err != nil {
			return struct{}{}, err
		}
		cfg["installationId"] = value
		encoded, err := json.Marshal(cfg)
		if err != nil {
			return struct{}{}, err
		}
		current.Config = models.JSONText(encoded)
		current.UpdatedAt = time.Now().UTC()
		return struct{}{}, transactional.IntegrationRepository.Update(tx, current)
	})
	if err != nil {
		return err
	}
	agent.IntegrationsChanged()
	return nil
}

// Inbound is the CodeHost side: a closed pull request ends the attempt it
// belongs to as merged or closed.
func (h *Host) Inbound(_ context.Context, in *models.Integration, r *http.Request) ([]agent.CodeHostEvent, error) {
	hook, err := h.parseRequest(in, r)
	if err != nil {
		return nil, err
	}
	if hook.event != "pull_request" || hook.action != "closed" {
		return nil, nil
	}
	return []agent.CodeHostEvent{{
		Link:   agent.Link{Provider: Provider, Kind: models.LinkKindPR, ExternalRef: issueRef(hook.owner, hook.repo, hook.number), IntegrationId: in.Id},
		Kind:   agent.CodeHostEventPullRequestClosed,
		Merged: hook.merged,
	}}, nil
}

// triggerView is Host as a TriggerSource: a labelled issue carrying a
// Traceway hash, or "@traceway fix" on such an issue, asks for an attempt.
type triggerView struct{ *Host }

func (v triggerView) Inbound(ctx context.Context, in *models.Integration, r *http.Request) ([]agent.Request, error) {
	hook, err := v.parseRequest(in, r)
	if err != nil {
		return nil, err
	}
	s, err := v.settings(in)
	if err != nil {
		return nil, err
	}
	var requested bool
	switch {
	case hook.event == "issues" && hook.action == "labeled" && !hook.isPR:
		requested = strings.EqualFold(hook.label, s.label())
	case hook.event == "issue_comment" && hook.action == "created" && !hook.isPR:
		requested = isFixCommand(hook.comment.body)
	}
	if !requested {
		return nil, nil
	}
	subject, ok, err := subjectFromIssue(in.OrganizationId, hook.body)
	if err != nil || !ok {
		return nil, err
	}
	author := hook.sender
	if hook.comment.user != "" {
		author = hook.comment.user
	}
	identity, err := identify(author)
	if err != nil {
		return nil, err
	}
	origin := agent.Link{Provider: Provider, Kind: models.LinkKindIssue, ExternalRef: issueRef(hook.owner, hook.repo, hook.number), URL: v.issueURL(hook.owner, hook.repo, hook.number), IntegrationId: in.Id}
	return []agent.Request{{Subject: subject, RequestedBy: identity, Origin: origin}}, nil
}

func isFixCommand(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		words := strings.Fields(strings.TrimSpace(line))
		if len(words) >= 2 && strings.EqualFold(words[0], mentionToken) && strings.EqualFold(words[1], "fix") {
			return true
		}
	}
	return false
}

// subjectFromIssue reads the exception hash and project the issue names on
// lines of their own, the way the github notification channel writes them,
// and checks the project belongs to the organization. Anything else in the
// issue is untrusted text.
func subjectFromIssue(organizationId int, body string) (agent.Subject, bool, error) {
	hashMatch := hashLineRe.FindStringSubmatch(body)
	projectMatch := projectLineRe.FindStringSubmatch(body)
	if hashMatch == nil || projectMatch == nil {
		return agent.Subject{}, false, nil
	}
	projectId, err := uuid.Parse(projectMatch[1])
	if err != nil {
		return agent.Subject{}, false, nil
	}
	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return transactional.ProjectRepository.FindById(tx, projectId)
	})
	if err != nil {
		return agent.Subject{}, false, err
	}
	if project == nil || project.OrganizationId == nil || *project.OrganizationId != organizationId {
		return agent.Subject{}, false, nil
	}
	return agent.Subject{Kind: models.SubjectKindTracewayException, Ref: hashMatch[1], ProjectId: project.Id}, true, nil
}

// identify maps a GitHub login onto a Traceway user through the identities
// an admin entered; GitHub exposes no email to match on. An unmapped login
// is an identity with no user, which the control plane ignores and records.
func identify(login string) (agent.Identity, error) {
	identity := agent.Identity{Provider: Provider, ExternalId: login, Display: login}
	if login == "" {
		return identity, nil
	}
	stored, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Identity, error) {
		return transactional.IdentityRepository.FindByExternalId(tx, Provider, strings.ToLower(login))
	})
	if err != nil {
		return identity, err
	}
	if stored != nil {
		identity.UserId = stored.UserId
		if stored.Display != "" {
			identity.Display = stored.Display
		}
	}
	return identity, nil
}

// channelView is Host as a Channel: the issue an attempt started from, or
// its pull request, is the thread; comments mentioning @traceway on it are
// replies.
type channelView struct{ *Host }

func (v channelView) Open(_ context.Context, in *models.Integration, attempt *models.AgentAttempt, origin *agent.Link) (agent.Link, error) {
	if origin == nil || origin.Provider != Provider || origin.ExternalRef == "" {
		return agent.Link{}, errors.New("github: an attempt gets a GitHub thread from the issue or pull request it started from")
	}
	return agent.Link{Provider: Provider, Kind: models.LinkKindThread, ExternalRef: origin.ExternalRef, URL: origin.URL, IntegrationId: in.Id}, nil
}

func (v channelView) Post(ctx context.Context, in *models.Integration, thread agent.Link, m agent.Message) (agent.Link, error) {
	if err := v.Comment(ctx, in, thread, mirrorText(m)); err != nil {
		return agent.Link{}, err
	}
	return thread, nil
}

func (v channelView) Inbound(ctx context.Context, in *models.Integration, r *http.Request) ([]agent.InboundMessage, error) {
	hook, err := v.parseRequest(in, r)
	if err != nil {
		return nil, err
	}
	if hook.action != "created" || (hook.event != "issue_comment" && hook.event != "pull_request_review_comment") {
		return nil, nil
	}
	if !strings.Contains(strings.ToLower(hook.comment.body), mentionToken) || isFixCommand(hook.comment.body) {
		return nil, nil
	}
	ref := issueRef(hook.owner, hook.repo, hook.number)
	thread, err := v.threadFor(in, ref)
	if err != nil || thread == nil {
		return nil, err
	}
	identity, err := identify(hook.comment.user)
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(strings.ReplaceAll(hook.comment.body, mentionToken, ""))
	return []agent.InboundMessage{{Thread: *thread, Author: identity, Body: body, ExternalRef: strconv.FormatInt(hook.comment.id, 10)}}, nil
}

// threadFor finds the link a comment belongs to: the pull request the
// attempt opened, or the issue thread it started from.
func (v channelView) threadFor(in *models.Integration, ref string) (*agent.Link, error) {
	return db.ExecuteTransaction(func(tx *sql.Tx) (*agent.Link, error) {
		for _, kind := range []string{models.LinkKindPR, models.LinkKindThread} {
			link, err := transactional.AgentLinkRepository.FindInbound(tx, in.Id, Provider, kind, ref)
			if err != nil {
				return nil, err
			}
			if link != nil {
				return &agent.Link{Provider: Provider, Kind: kind, ExternalRef: ref, URL: link.URL, IntegrationId: in.Id}, nil
			}
		}
		return nil, nil
	})
}

func (h *Host) issueURL(owner, repo string, number int) string {
	return fmt.Sprintf("%s/%s/%s/issues/%d", strings.TrimRight(h.WebBase, "/"), owner, repo, number)
}

// mirrorText renders a thread entry as a GitHub comment: the agent's own
// entries by kind, everyone else's with the surface they came from.
func mirrorText(m agent.Message) string {
	body := m.Body
	if m.Provider == agent.ProviderAgent {
		switch m.Kind {
		case models.MessageKindQuestion:
			return "**The agent needs an answer.** Reply with `@traceway` and your answer.\n\n" + body
		case models.MessageKindFinding:
			return "**Finding**\n\n" + body
		}
		return body
	}
	who := m.Provider
	if m.Author != nil && m.Author.Display != "" {
		who = m.Author.Display + " via " + m.Provider
	}
	return "_" + who + "_\n\n" + body
}
