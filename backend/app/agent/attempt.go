package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// StartOptions is what a surface knows when it asks for an attempt.
type StartOptions struct {
	ProfileId       *int
	RequireApproval bool
	Origin          *Link
	RequestedBy     *int
}

// StartResult is the attempt a start produced, or the active one that
// already existed for the subject.
type StartResult struct {
	Attempt  *models.AgentAttempt
	Existing bool
}

var ErrProjectHasNoOrganization = errors.New("project belongs to no organization")

var StartLimitHook func(tx *sql.Tx, organizationId int) error

// StartAttempt is the one way an attempt begins, whichever surface asked.
// An active attempt for the subject is returned instead of a new one; the
// new one gets the next number for the subject, the project's repository
// and the chosen or default profile, and waits for approval when asked.
// Callers wake the executors after their transaction commits.
func StartAttempt(tx *sql.Tx, project *models.Project, subject Subject, opts StartOptions) (*StartResult, error) {
	if project.OrganizationId == nil {
		return nil, ErrProjectHasNoOrganization
	}
	if subject.ProjectId != project.Id {
		return nil, errors.New("subject belongs to another project")
	}
	if opts.RequestedBy != nil {
		role, err := transactional.ProjectRepository.GetEffectiveRole(tx, project.Id, *opts.RequestedBy)
		if err != nil {
			return nil, err
		}
		if role == "" || role == "readonly" {
			return nil, errInboundUnauthorized
		}
	}
	existing, err := transactional.AgentAttemptRepository.FindActiveBySubject(tx, project.Id, subject.Kind, subject.Ref)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return &StartResult{Attempt: existing, Existing: true}, nil
	}
	if StartLimitHook != nil {
		if err := StartLimitHook(tx, *project.OrganizationId); err != nil {
			return nil, err
		}
	}

	repository, err := transactional.RepositoryRepository.FindByProject(tx, project.Id)
	if err != nil {
		return nil, err
	}
	profile, err := resolveProfile(tx, *project.OrganizationId, opts.ProfileId)
	if err != nil {
		return nil, err
	}
	number, err := transactional.AgentAttemptRepository.NextNumber(tx, project.Id, subject.Kind, subject.Ref)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	status := models.AttemptQueued
	if opts.RequireApproval {
		status = models.AttemptPendingApproval
	}
	attempt := &models.AgentAttempt{
		Id:             uuid.New(),
		OrganizationId: *project.OrganizationId,
		ProjectId:      project.Id,
		Number:         number,
		Kind:           models.AttemptKindFix,
		SubjectKind:    subject.Kind,
		SubjectRef:     subject.Ref,
		Status:         status,
		RequestedBy:    opts.RequestedBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if repository != nil {
		attempt.RepositoryId = &repository.Id
		attempt.BaseBranch = repository.DefaultBranch
	}
	if profile != nil {
		attempt.ProfileId = &profile.Id
		attempt.Agent = profile.Agent
		attempt.Model = profile.Model
	}
	if err := transactional.AgentAttemptRepository.Create(tx, attempt); err != nil {
		return nil, err
	}
	if opts.Origin != nil {
		if _, err := RecordLink(tx, attempt.Id, *opts.Origin, now); err != nil {
			return nil, err
		}
	}
	payload := map[string]any{"number": number, "status": status, "subjectKind": subject.Kind, "subjectRef": subject.Ref}
	if opts.Origin != nil {
		payload["origin"] = opts.Origin.Provider
	}
	if _, err := AppendEvent(tx, attempt.Id, EventCreated, payload, now); err != nil {
		return nil, err
	}
	recordStarted()
	if err := queueThreads(tx, attempt, opts.Origin); err != nil {
		return nil, err
	}
	return &StartResult{Attempt: attempt}, nil
}

func resolveProfile(tx *sql.Tx, organizationId int, profileId *int) (*models.AgentProfile, error) {
	if profileId == nil {
		return transactional.AgentProfileRepository.FindDefault(tx, organizationId)
	}
	profile, err := transactional.AgentProfileRepository.FindById(tx, *profileId)
	if err != nil {
		return nil, err
	}
	if profile == nil || profile.OrganizationId != organizationId {
		return nil, fmt.Errorf("agent profile %d not found in organization %d", *profileId, organizationId)
	}
	return profile, nil
}

// RecordLink attaches an external artifact to an attempt. Recording the
// same artifact twice is not an error: the existing row is returned.
func RecordLink(tx *sql.Tx, attemptId uuid.UUID, link Link, now time.Time) (*models.AgentLink, error) {
	existing, err := transactional.AgentLinkRepository.FindForAttempt(tx, attemptId, link.IntegrationId, link.Provider, link.Kind, link.ExternalRef)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	row := &models.AgentLink{AttemptId: attemptId, Provider: link.Provider, Kind: link.Kind, ExternalRef: link.ExternalRef, URL: link.URL, CreatedAt: now}
	if link.IntegrationId != 0 {
		row.IntegrationId = &link.IntegrationId
	}
	id, err := transactional.AgentLinkRepository.Create(tx, row)
	if err != nil {
		return nil, err
	}
	row.Id = id
	return row, nil
}

// Approve releases a pending attempt into the queue. False means it was not
// pending (already approved, or cancelled).
func Approve(tx *sql.Tx, id uuid.UUID, userId int, now time.Time) (bool, error) {
	approved, err := transactional.AgentAttemptRepository.Approve(tx, id, userId, now)
	if err != nil || !approved {
		return approved, err
	}
	_, err = AppendEvent(tx, id, EventStatus, map[string]any{"status": models.AttemptQueued, "approvedBy": userId}, now)
	return true, err
}

// Cancel ends an attempt from any active status. False means it was already
// terminal.
func Cancel(tx *sql.Tx, id uuid.UUID, userId int, now time.Time) (bool, error) {
	err := Transition(tx, id, models.AttemptCancelled, now)
	if errors.Is(err, ErrIllegalTransition) {
		return false, nil
	}
	return err == nil, err
}

// AppendMessage stores one thread entry. An inbound message on an attempt
// that is waiting (for input, or for review of its pull request) sends it
// back to the queue with resume set, so the executor continues the session
// with the reply.
func AppendMessage(tx *sql.Tx, m Message, link *models.AgentLink, externalRef string, now time.Time) (*models.AgentMessage, error) {
	row := &models.AgentMessage{
		AttemptId:   m.AttemptId,
		Direction:   m.Direction,
		Provider:    m.Provider,
		Kind:        m.Kind,
		Body:        m.Body,
		ExternalRef: externalRef,
		CreatedAt:   now,
	}
	if link != nil {
		row.LinkId = &link.Id
	}
	if m.Author != nil && m.Author.UserId != 0 {
		var identity *models.Identity
		var err error
		if m.Author.ExternalId != "" {
			identity, err = transactional.IdentityRepository.FindByExternalId(tx, m.Provider, m.Author.ExternalId)
		} else {
			identity, err = transactional.IdentityRepository.FindByUserAndProvider(tx, m.Author.UserId, m.Provider)
		}
		if err != nil {
			return nil, err
		}
		if identity != nil && identity.UserId == m.Author.UserId {
			row.IdentityId = &identity.Id
		}
	}
	id, err := transactional.AgentMessageRepository.Create(tx, row)
	if err != nil {
		return nil, err
	}
	row.Id = id
	if _, err := AppendEvent(tx, m.AttemptId, EventMessage, map[string]any{"messageId": id, "direction": m.Direction, "provider": m.Provider, "kind": m.Kind}, now); err != nil {
		return nil, err
	}
	if m.Direction != models.MessageInbound {
		return row, nil
	}
	attempt, err := transactional.AgentAttemptRepository.FindById(tx, m.AttemptId)
	if err != nil {
		return nil, err
	}
	if attempt == nil {
		return nil, fmt.Errorf("attempt %s not found", m.AttemptId)
	}
	if attempt.Status == models.AttemptNeedsInput || attempt.Status == models.AttemptAwaitingReview {
		if err := transactional.AgentAttemptRepository.SetResume(tx, attempt.Id, true, now); err != nil {
			return nil, err
		}
		if err := Transition(tx, attempt.Id, models.AttemptQueued, now); err != nil {
			return nil, err
		}
	}
	return row, nil
}

// Blob keys under which an attempt's transcript, diff and report rest in
// storage; the retention worker deletes exactly these.
const (
	BlobTranscript = "transcript.jsonl"
	BlobDiff       = "diff.patch"
	BlobReport     = "report.md"
)

func BlobKey(attemptId uuid.UUID, name string) string {
	return "agent/" + attemptId.String() + "/" + name
}

func AttemptBlobKeys(attemptId uuid.UUID) []string {
	return []string{BlobKey(attemptId, BlobTranscript), BlobKey(attemptId, BlobDiff), BlobKey(attemptId, BlobReport)}
}

// RequestApproval asks every enabled chat integration of the attempt's
// organization that can ask for approval to do so, recording the thread
// each one opens. It runs outside the starting transaction because the
// channels talk to their providers.
func RequestApproval(ctx context.Context, attempt *models.AgentAttempt) error {
	integrations, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Integration, error) {
		return transactional.IntegrationRepository.FindByOrganization(tx, attempt.OrganizationId)
	})
	if err != nil {
		return err
	}
	for _, in := range integrations {
		if !in.Enabled {
			continue
		}
		channel, ok := ChannelFor(in.Provider)
		if !ok {
			continue
		}
		_, ok = channel.(ApprovalRequester)
		if !ok {
			continue
		}
		cfg, err := json.Marshal(threadOpening{SchemaVersion: EventSchemaVersion, AttemptId: attempt.Id, IntegrationId: in.Id})
		if err != nil {
			return err
		}
		if err := deliverThreadOpening(ctx, cfg); err != nil {
			return err
		}
	}
	return nil
}
