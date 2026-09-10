package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/outbox"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// ProviderAgent marks thread entries the agent itself wrote; every other
// provider value names the surface a message came from.
const ProviderAgent = "agent"

const adapterTypePrefix = "agent:"

// mirrorDelivery is the outbox snapshot of one mirrored message. It is a
// wire format: rows in flight survive restarts.
type mirrorDelivery struct {
	SchemaVersion int     `json:"schemaVersion"`
	IntegrationId int     `json:"integrationId"`
	Thread        Link    `json:"thread"`
	Message       Message `json:"message"`
}

// Mirror enqueues the message on every linked thread of another surface
// than the one it came from, through the at-least-once outbox. Providers
// must deduplicate retries where their API supports it. Surfaces without an integration (the
// dashboard) need nothing: the row in agent_messages is their thread.
func Mirror(tx *sql.Tx, attempt *models.AgentAttempt, m Message) error {
	links, err := transactional.AgentLinkRepository.FindByAttempt(tx, attempt.Id)
	if err != nil {
		return err
	}
	for _, link := range links {
		if (link.Kind != models.LinkKindThread && link.Kind != models.LinkKindPR) || link.Provider == m.Provider || link.IntegrationId == nil {
			continue
		}
		if _, ok := ChannelFor(link.Provider); !ok {
			continue
		}
		if err := enqueueMirror(tx, attempt, link, m); err != nil {
			return err
		}
	}
	return nil
}

func enqueueMirror(tx *sql.Tx, attempt *models.AgentAttempt, link *models.AgentLink, m Message) error {
	snapshot, err := json.Marshal(mirrorDelivery{
		SchemaVersion: EventSchemaVersion,
		IntegrationId: *link.IntegrationId,
		Thread:        Link{Provider: link.Provider, Kind: link.Kind, ExternalRef: link.ExternalRef, URL: link.URL, IntegrationId: *link.IntegrationId},
		Message:       m,
	})
	if err != nil {
		return err
	}
	projectId := attempt.ProjectId
	if _, err := outbox.Enqueue(tx, outbox.Delivery{
		Kind:          models.OutboxKindAgent,
		AdapterType:   adapterTypePrefix + link.Provider,
		AdapterConfig: snapshot,
		Message:       models.NotificationMessage{Subject: fmt.Sprintf("Attempt %d", attempt.Number), Body: m.Body, Severity: models.NotificationSeverityInfo},
		ProjectId:     &projectId,
		ChannelName:   link.Provider,
	}); err != nil {
		return err
	}
	return nil
}

// OutboxSender wraps the notification sender so deliveries addressed to a
// channel provider reach its Post; everything else passes through.
func OutboxSender(next outbox.SendFunc) outbox.SendFunc {
	return func(ctx context.Context, adapterType string, adapterConfig json.RawMessage, msg models.NotificationMessage) error {
		if adapterType == openThreadAdapter {
			return deliverThreadOpening(ctx, adapterConfig)
		}
		if !strings.HasPrefix(adapterType, adapterTypePrefix) {
			return next(ctx, adapterType, adapterConfig, msg)
		}
		provider := strings.TrimPrefix(adapterType, adapterTypePrefix)
		channel, ok := ChannelFor(provider)
		if !ok {
			return fmt.Errorf("agent: no channel registered for provider %s", provider)
		}
		var delivery mirrorDelivery
		if err := json.Unmarshal(adapterConfig, &delivery); err != nil {
			return fmt.Errorf("agent: decode mirror delivery: %w", err)
		}
		integration, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Integration, error) {
			return transactional.IntegrationRepository.FindById(tx, delivery.IntegrationId)
		})
		if err != nil {
			return err
		}
		if integration == nil || !integration.Enabled {
			return fmt.Errorf("agent: integration %d for %s is gone or disabled", delivery.IntegrationId, provider)
		}
		_, err = channel.Post(ctx, integration, delivery.Thread, delivery.Message)
		return err
	}
}

// Post records a message and mirrors it to the other surfaces in one
// transaction; the caller wakes the queue after commit when the message
// resumed the attempt.
func Post(tx *sql.Tx, attempt *models.AgentAttempt, m Message, link *models.AgentLink, externalRef string, now time.Time) (*models.AgentMessage, error) {
	m.AttemptId = attempt.Id
	m.CreatedAt = now
	row, err := AppendMessage(tx, m, link, externalRef, now)
	if err != nil {
		return nil, err
	}
	return row, Mirror(tx, attempt, m)
}

// InboundResult counts what one provider request produced. Response is set
// when the provider answered a transport handshake instead, and is what the
// HTTP route writes back verbatim.
type InboundResult struct {
	Requests int    `json:"requests"`
	Messages int    `json:"messages"`
	Events   int    `json:"events"`
	Ignored  int    `json:"ignored"`
	Response []byte `json:"-"`
}

// Inbound is everything one provider delivery decoded into, whichever
// transport carried it.
type Inbound struct {
	Requests []Request
	Messages []InboundMessage
	Events   []CodeHostEvent
}

// InboundResponder is a provider whose transport needs an answer of its own
// before any event is dispatched, such as Slack's URL verification
// challenge. A handled request produces no events.
type InboundResponder interface {
	Respond(in *models.Integration, r *http.Request) (response []byte, handled bool, err error)
}

var errInboundUnauthorized = errors.New("author is not a member with access")

// HandleInbound decodes one signed provider request through every port the
// provider implements and dispatches the result. The provider verified the
// signature; Dispatch decides authorization through identities and project
// roles.
func HandleInbound(ctx context.Context, provider string, integration *models.Integration, r *http.Request) (InboundResult, error) {
	if responder, ok := responderFor(provider); ok {
		response, handled, err := responder.Respond(integration, r)
		if err != nil {
			return InboundResult{}, err
		}
		if handled {
			return InboundResult{Response: response}, nil
		}
	}

	var in Inbound
	if trigger, ok := TriggerFor(provider); ok {
		requests, err := trigger.Inbound(ctx, integration, r)
		if err != nil {
			return InboundResult{}, err
		}
		in.Requests = requests
	}
	if channel, ok := ChannelFor(provider); ok {
		messages, err := channel.Inbound(ctx, integration, r)
		if err != nil {
			return InboundResult{}, err
		}
		in.Messages = messages
	}
	if host, ok := CodeHostFor(provider); ok {
		events, err := host.Inbound(ctx, integration, r)
		if err != nil {
			return InboundResult{}, err
		}
		in.Events = events
	}
	return Dispatch(ctx, provider, integration, in)
}

func responderFor(provider string) (InboundResponder, bool) {
	if trigger, ok := TriggerFor(provider); ok {
		if responder, ok := trigger.(InboundResponder); ok {
			return responder, true
		}
	}
	if channel, ok := ChannelFor(provider); ok {
		if responder, ok := channel.(InboundResponder); ok {
			return responder, true
		}
	}
	return nil, false
}

// Dispatch routes decoded provider input to the attempt machinery: trigger
// requests start attempts (and open the requesting channel's thread),
// channel replies become thread messages that resume the attempt, code host
// events close the loop on a pull request. Transports that do not arrive as
// an HTTP request (Socket Mode) call this directly.
func Dispatch(ctx context.Context, provider string, integration *models.Integration, in Inbound) (InboundResult, error) {
	var result InboundResult
	now := time.Now().UTC()

	for _, request := range in.Requests {
		attempt, err := startFromRequest(integration, request, now)
		if err != nil {
			if errors.Is(err, errInboundUnauthorized) {
				result.Ignored++
				continue
			}
			return result, err
		}
		if attempt == nil {
			continue
		}
		if err := openThread(ctx, provider, integration, attempt, request.Origin); err != nil {
			return result, err
		}
		result.Requests++
		Wake()
	}

	for _, inbound := range in.Messages {
		accepted, err := acceptInboundMessage(integration, provider, inbound, now)
		if err != nil {
			return result, err
		}
		if !accepted {
			result.Ignored++
			continue
		}
		result.Messages++
		Wake()
	}

	for _, event := range in.Events {
		applied, err := applyCodeHostEvent(ctx, integration, event, now)
		if err != nil {
			return result, err
		}
		if applied {
			result.Events++
		} else {
			result.Ignored++
		}
	}
	return result, nil
}

// startFromRequest starts the attempt a request asks for and returns it, or
// nil when an active attempt already covers the subject.
func startFromRequest(integration *models.Integration, request Request, now time.Time) (*models.AgentAttempt, error) {
	if request.RequestedBy.UserId == 0 {
		return nil, errInboundUnauthorized
	}
	return db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		project, err := transactional.ProjectRepository.FindById(tx, request.Subject.ProjectId)
		if err != nil {
			return nil, err
		}
		if project == nil || project.OrganizationId == nil || *project.OrganizationId != integration.OrganizationId {
			return nil, errInboundUnauthorized
		}
		role, err := transactional.ProjectRepository.GetEffectiveRole(tx, project.Id, request.RequestedBy.UserId)
		if err != nil {
			return nil, err
		}
		if role == "" || role == "readonly" {
			return nil, errInboundUnauthorized
		}
		origin := request.Origin
		origin.IntegrationId = integration.Id
		userId := request.RequestedBy.UserId
		started, err := StartAttempt(tx, project, request.Subject, StartOptions{RequireApproval: request.RequireApproval, Origin: &origin, RequestedBy: &userId})
		if err != nil {
			return nil, err
		}
		if started.Existing {
			return nil, nil
		}
		return started.Attempt, nil
	})
}

// openThread gives a freshly started attempt its thread on the surface that
// requested it. The channel's Open talks to the provider, so it runs outside
// any transaction and the link is recorded afterwards.
func openThread(ctx context.Context, provider string, integration *models.Integration, attempt *models.AgentAttempt, origin Link) error {
	_, ok := ChannelFor(provider)
	if !ok {
		return nil
	}
	cfg, err := json.Marshal(threadOpening{SchemaVersion: EventSchemaVersion, AttemptId: attempt.Id, IntegrationId: integration.Id, Origin: &origin})
	if err != nil {
		return err
	}
	return deliverThreadOpening(ctx, cfg)
}

func acceptInboundMessage(integration *models.Integration, provider string, inbound InboundMessage, now time.Time) (bool, error) {
	if inbound.Author.UserId == 0 {
		return false, nil
	}
	return db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		link, err := transactional.AgentLinkRepository.FindInbound(tx, integration.Id, provider, inbound.Thread.Kind, inbound.Thread.ExternalRef)
		if err != nil {
			return false, err
		}
		if link == nil {
			return false, nil
		}
		if inbound.ExternalRef != "" {
			seen, err := transactional.AgentMessageRepository.FindByExternalRef(tx, link.AttemptId, provider, inbound.ExternalRef)
			if err != nil {
				return false, err
			}
			if seen != nil {
				return false, nil
			}
		}
		attempt, err := transactional.AgentAttemptRepository.FindById(tx, link.AttemptId)
		if err != nil {
			return false, err
		}
		if attempt == nil || attempt.OrganizationId != integration.OrganizationId {
			return false, nil
		}
		role, err := transactional.ProjectRepository.GetEffectiveRole(tx, attempt.ProjectId, inbound.Author.UserId)
		if err != nil {
			return false, err
		}
		if role == "" || role == "readonly" {
			return false, nil
		}
		author := inbound.Author
		_, err = Post(tx, attempt, Message{Direction: models.MessageInbound, Provider: provider, Kind: models.MessageKindAnswer, Body: inbound.Body, Author: &author}, link, inbound.ExternalRef, now)
		return err == nil, err
	})
}

func applyCodeHostEvent(ctx context.Context, integration *models.Integration, event CodeHostEvent, now time.Time) (bool, error) {
	if event.Kind != CodeHostEventPullRequestClosed {
		return false, nil
	}
	return db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		if event.Link.Provider != integration.Provider {
			return false, nil
		}
		link, err := transactional.AgentLinkRepository.FindInbound(tx, integration.Id, event.Link.Provider, event.Link.Kind, event.Link.ExternalRef)
		if err != nil {
			return false, err
		}
		if link == nil {
			return false, nil
		}
		to := models.AttemptClosed
		attempt, err := transactional.AgentAttemptRepository.FindById(tx, link.AttemptId)
		if err != nil {
			return false, err
		}
		if attempt == nil || attempt.OrganizationId != integration.OrganizationId {
			return false, nil
		}
		if event.Merged {
			to = models.AttemptMerged
		}
		err = Transition(tx, link.AttemptId, to, now)
		if errors.Is(err, ErrIllegalTransition) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if event.Merged {
			return true, archiveSubject(ctx, tx, link.AttemptId)
		}
		return true, nil
	})
}

// archiveSubject closes the loop on a merged fix: the exception the
// attempt was about is archived like a hand-resolved one.
func archiveSubject(ctx context.Context, tx *sql.Tx, attemptId uuid.UUID) error {
	attempt, err := transactional.AgentAttemptRepository.FindById(tx, attemptId)
	if err != nil || attempt == nil || attempt.SubjectKind != models.SubjectKindTracewayException {
		return err
	}
	return telemetry.ExceptionStackTraceRepository.ArchiveByHashes(ctx, attempt.ProjectId, []string{attempt.SubjectRef})
}

// LoadIntegration resolves the integration an inbound route names, enabled
// and of the expected provider, or nil.
func LoadIntegration(provider string, id int) (*models.Integration, error) {
	integration, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Integration, error) {
		return transactional.IntegrationRepository.FindById(tx, id)
	})
	if err != nil {
		return nil, err
	}
	if integration == nil || integration.Provider != provider || !integration.Enabled {
		return nil, nil
	}
	return integration, nil
}

// FindAttemptForRunToken resolves the attempt behind a run token and checks
// it is still active, for the executor-facing endpoints.
func FindAttemptForRunToken(tx *sql.Tx, attemptId uuid.UUID) (*models.AgentAttempt, error) {
	attempt, err := transactional.AgentAttemptRepository.FindById(tx, attemptId)
	if err != nil {
		return nil, err
	}
	if attempt == nil || !models.AttemptIsActive(attempt.Status) {
		return nil, ErrAttemptNotActive
	}
	return attempt, nil
}
