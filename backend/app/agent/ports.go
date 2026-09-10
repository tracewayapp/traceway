// Package agent is the control plane of the Traceway fix agent: attempts,
// their state machine and queue, the conversation thread, context packs and
// run tokens. Everything that varies (where a request comes from, where the
// conversation is mirrored, where the code lives, what evidence a subject
// carries) sits behind the ports in this file and is registered by name;
// this package imports no provider.
package agent

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
)

// Subject is what an attempt fixes: a kind plus a provider ref inside a
// project. Stage 1 knows traceway_exception, whose ref is an exception hash.
type Subject struct {
	Kind      string    `json:"kind"`
	Ref       string    `json:"ref"`
	ProjectId uuid.UUID `json:"projectId"`
}

// Identity is who acted, resolved to a Traceway user when the provider
// account is mapped; UserId is zero for an unmapped external author.
type Identity struct {
	UserId     int    `json:"userId,omitempty"`
	Provider   string `json:"provider"`
	ExternalId string `json:"externalId,omitempty"`
	Display    string `json:"display,omitempty"`
}

// Link is an external artifact of an attempt: its origin, a chat thread, an
// issue, a pull request, an alert. Kind values are the models.LinkKind*
// constants.
type Link struct {
	Provider      string `json:"provider"`
	Kind          string `json:"kind"`
	ExternalRef   string `json:"externalRef"`
	URL           string `json:"url,omitempty"`
	IntegrationId int    `json:"integrationId,omitempty"`
}

// Message is one thread entry as a channel sees it.
type Message struct {
	AttemptId uuid.UUID `json:"attemptId"`
	Direction string    `json:"direction"`
	Provider  string    `json:"provider"`
	Kind      string    `json:"kind"`
	Body      string    `json:"body"`
	Author    *Identity `json:"author,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// InboundMessage is a reply a provider received in a thread it opened.
type InboundMessage struct {
	Thread      Link
	Author      Identity
	Body        string
	ExternalRef string
}

// DefaultChannel can open a conversation without an originating provider thread.
type DefaultChannel interface {
	CanOpenDefaultThread() bool
}

// Request is a provider asking for an attempt on a subject.
type Request struct {
	Subject         Subject
	RequestedBy     Identity
	Origin          Link
	RequireApproval bool
}

// CodeHostEvent is something that happened to a linked artifact at the code
// host, such as a pull request being merged or closed.
type CodeHostEvent struct {
	Link   Link
	Kind   string
	Merged bool
}

const (
	CodeHostEventPullRequestClosed = "pull_request_closed"
)

// Field describes one setting of a provider so the settings page can render
// the form without provider-specific code.
type Field struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Kind     string   `json:"kind"`
	Help     string   `json:"help,omitempty"`
	Options  []string `json:"options,omitempty"`
	Required bool     `json:"required"`
}

const (
	FieldText   = "text"
	FieldSecret = "secret"
	FieldSelect = "select"
	FieldURL    = "url"
)

// SetupFlow is an out-of-band setup step (an OAuth install, a manifest
// flow) the settings page opens in a new tab.
type SetupFlow struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Provider is what every port shares: a name, the integration kinds it
// fulfils, the fields its integration row needs, and how to validate them.
type Provider interface {
	Provider() string
	Kinds() []string
	Fields() []Field
	SetupFlow() *SetupFlow
	Validate(config map[string]string) error
}

const (
	KindChat         = "chat"
	KindCodeHost     = "code_host"
	KindIssueTracker = "issue_tracker"
	KindTrigger      = "trigger"
	KindTelemetry    = "telemetry"
)

// ContextSection is the evidence a subject contributes to the context pack.
// Summary holds only validated fields and may be quoted in instructions;
// Data is untrusted provider text and is only ever rendered inside a
// delimited data block.
type ContextSection struct {
	Title   string         `json:"title"`
	Summary string         `json:"summary"`
	Data    map[string]any `json:"data,omitempty"`
	Links   []Link         `json:"links,omitempty"`
}

// ContextProvider builds the evidence for one subject kind. Telemetry is
// read with the attempt's own run token so the pack never contains more
// than the agent could read itself.
type ContextProvider interface {
	Kind() string
	Build(ctx context.Context, subject Subject, runToken string) (ContextSection, error)
}

// TriggerSource turns a provider's inbound request into attempt requests.
type TriggerSource interface {
	Provider
	Inbound(ctx context.Context, in *models.Integration, r *http.Request) ([]Request, error)
}

// Channel mirrors an attempt's thread to a surface and reads replies back.
type Channel interface {
	Provider
	Open(ctx context.Context, in *models.Integration, attempt *models.AgentAttempt, origin *Link) (Link, error)
	Post(ctx context.Context, in *models.Integration, thread Link, m Message) (Link, error)
	Inbound(ctx context.Context, in *models.Integration, r *http.Request) ([]InboundMessage, error)
}

// GitCredential is a short-lived credential the harness uses outside the
// sandbox to clone and push; the agent process never sees it.
type GitCredential struct {
	Username  string
	Password  string
	ExpiresAt time.Time
}

type PullRequest struct {
	Title string
	Body  string
	Head  string
	Base  string
	Draft bool
}

// CodeHost is where the repository lives and the pull request goes.
type CodeHost interface {
	Provider
	CloneURL(repo *models.Repository) string
	CloneCredential(ctx context.Context, in *models.Integration, repo *models.Repository) (GitCredential, error)
	OpenPullRequest(ctx context.Context, in *models.Integration, repo *models.Repository, pr PullRequest) (Link, error)
	Comment(ctx context.Context, in *models.Integration, target Link, body string) error
	Inbound(ctx context.Context, in *models.Integration, r *http.Request) ([]CodeHostEvent, error)
}

type Issue struct {
	Title string
	Body  string
}

// IssueTracker is where the work item lives.
type IssueTracker interface {
	Provider
	OpenIssue(ctx context.Context, in *models.Integration, repo *models.Repository, issue Issue) (Link, error)
	Comment(ctx context.Context, in *models.Integration, target Link, body string) error
}

// ApprovalRequester is a channel that can ask people to approve a pending
// attempt where they are (a Slack message with an Approve button). The
// link it returns becomes the attempt's thread on that surface.
type ApprovalRequester interface {
	RequestApproval(ctx context.Context, in *models.Integration, attempt *models.AgentAttempt) (Link, error)
}

// Describer is a provider that can say, in a few words, where an
// integration stands (a GitHub App created but not installed); the
// repository tab shows it next to the integration.
type Describer interface {
	Describe(in *models.Integration) string
}

// Executor is somewhere attempts run: the embedded harness, a remote runner
// fleet, CI. Available answers the preflight's executor check.
type Executor interface {
	Name() string
	Available(ctx context.Context, tx *sql.Tx) (bool, string)
}
