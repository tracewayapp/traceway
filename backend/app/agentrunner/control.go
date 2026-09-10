package agentrunner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/agentrunner/agents"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/secrets"
	"github.com/tracewayapp/traceway/backend/app/storage"
)

// ProtocolVersion is the schemaVersion of the runner protocol's wire types.
const ProtocolVersion = 1

// Inputs is everything a run needs from the control plane, resolved once
// before the workspace is prepared: the rows, the rendered context pack,
// the provider credential and a run token. Remote runners fetch it over
// the runner protocol, so it is a wire format.
type Inputs struct {
	SchemaVersion     int                  `json:"schemaVersion"`
	Attempt           *models.AgentAttempt `json:"attempt"`
	Repository        *models.Repository   `json:"repository"`
	CloneURL          string               `json:"cloneUrl"`
	Profile           agents.Profile       `json:"profile"`
	Pack              string               `json:"pack"`
	RunToken          string               `json:"runToken"`
	RunTokenExpiresAt time.Time            `json:"runTokenExpiresAt"`
	SessionId         string               `json:"sessionId,omitempty"`
}

// Outcome is how a run ended, as the harness reports it: the branch it
// pushed and the report for a fix, the report alone for an analysis or a
// question, the reason for an error. The control plane turns it into the
// pull request, the thread message and the final status.
type Outcome struct {
	SchemaVersion int          `json:"schemaVersion"`
	Status        string       `json:"status"`
	Branch        string       `json:"branch,omitempty"`
	Report        string       `json:"report,omitempty"`
	Usage         agents.Usage `json:"usage"`
	Error         string       `json:"error,omitempty"`
}

// Claimer hands the harness attempts and what they need. The embedded
// executor reads the database; a remote runner asks the backend.
type Claimer interface {
	Claim(ctx context.Context, limit int) ([]*models.AgentAttempt, error)
	Inputs(ctx context.Context, attempt *models.AgentAttempt) (*Inputs, error)
	GitCredential(ctx context.Context, attempt *models.AgentAttempt) (agent.GitCredential, error)
}

// Reporter is the control-plane side of a run: status, events with the
// lease heartbeat, thread findings, blobs and the terminal outcome.
type Reporter interface {
	Transition(ctx context.Context, attemptId uuid.UUID, to string) error
	Renew(ctx context.Context, attemptId uuid.UUID) (bool, error)
	Events(ctx context.Context, attemptId uuid.UUID, batch []agents.Event) error
	Finding(ctx context.Context, attemptId uuid.UUID, body string) error
	Blob(ctx context.Context, attemptId uuid.UUID, name string, content []byte, appendTo bool) error
	Finish(ctx context.Context, attemptId uuid.UUID, outcome Outcome) error
}

// ErrClaimLost is returned when the attempt is no longer this executor's:
// cancelled, reclaimed after a lease expiry, or finished elsewhere.
var ErrClaimLost = errors.New("the attempt's claim was lost")

// Local is the control plane in-process: what the embedded executor uses
// directly, and what the runner protocol's server side calls on behalf of
// a remote runner. Executor is the claim identity it acts as.
type Local struct {
	Executor    string
	ClaimID     string
	InstanceURL string
}

type scopedControl interface {
	ForAttempt(*models.AgentAttempt) (Claimer, Reporter)
}

func (l Local) ForAttempt(attempt *models.AgentAttempt) (Claimer, Reporter) {
	l.ClaimID = attempt.ClaimedBy
	return l, l
}

func (l Local) claimed(tx *sql.Tx, id uuid.UUID) (*models.AgentAttempt, error) {
	if l.ClaimID == "" {
		return nil, ErrClaimLost
	}
	held, err := agent.Renew(tx, id, l.ClaimID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if !held {
		return nil, ErrClaimLost
	}
	return transactional.AgentAttemptRepository.FindById(tx, id)
}

func (l Local) Claim(ctx context.Context, limit int) ([]*models.AgentAttempt, error) {
	return db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.AgentAttempt, error) {
		return agent.Claim(tx, l.Executor, limit, time.Now().UTC())
	})
}

func (l Local) Inputs(ctx context.Context, attempt *models.AgentAttempt) (*Inputs, error) {
	return db.ExecuteTransaction(func(tx *sql.Tx) (*Inputs, error) {
		attempt, err := l.claimed(tx, attempt.Id)
		if err != nil {
			return nil, err
		}
		repository, integration, err := repositoryOf(tx, attempt)
		if err != nil {
			return nil, err
		}
		host, ok := agent.CodeHostFor(integration.Provider)
		if !ok {
			return nil, fmt.Errorf("no code host registered for %s", integration.Provider)
		}
		if attempt.ProfileId == nil {
			return nil, errors.New("the attempt has no agent profile")
		}
		profile, err := transactional.AgentProfileRepository.FindById(tx, *attempt.ProfileId)
		if err != nil {
			return nil, err
		}
		if profile == nil {
			return nil, errors.New("the agent profile is gone")
		}
		credential := ""
		if profile.Credential != "" {
			plaintext, err := secrets.Decrypt(profile.Credential)
			if err != nil {
				return nil, fmt.Errorf("decrypt the profile credential: %w", err)
			}
			credential = string(plaintext)
		}
		token, err := agent.MintRunToken(tx, attempt.Id)
		if err != nil {
			return nil, err
		}
		if _, err := agent.AppendEvent(tx, attempt.Id, agent.EventCredential, map[string]any{"kind": "run_token", "executor": l.Executor, "expiresAt": token.ExpiresAt}, time.Now().UTC()); err != nil {
			return nil, err
		}
		pack, err := agent.BuildContextPack(ctx, tx, attempt, token.Token, l.InstanceURL)
		if err != nil {
			return nil, err
		}
		inputs := &Inputs{
			SchemaVersion:     ProtocolVersion,
			Attempt:           attempt,
			Repository:        repository,
			CloneURL:          host.CloneURL(repository),
			Profile:           agents.Profile{Model: profile.Model, Provider: profile.Provider, BaseURL: profile.BaseURL, Credential: credential, MaxTurns: profile.MaxTurns, BudgetUSD: profile.BudgetUSD, TimeoutMinutes: profile.TimeoutMinutes, AllowedTools: profile.AllowedTools},
			Pack:              pack.Render(),
			RunToken:          token.Token,
			RunTokenExpiresAt: token.ExpiresAt,
		}
		if attempt.Resume {
			inputs.SessionId, err = latestSessionId(tx, attempt.Id)
			if err != nil {
				return nil, err
			}
		}
		return inputs, nil
	})
}

func (l Local) GitCredential(ctx context.Context, attempt *models.AgentAttempt) (agent.GitCredential, error) {
	type target struct {
		repository  *models.Repository
		integration *models.Integration
	}
	loaded, err := db.ExecuteTransaction(func(tx *sql.Tx) (target, error) {
		attempt, err := l.claimed(tx, attempt.Id)
		if err != nil {
			return target{}, err
		}
		repository, integration, err := repositoryOf(tx, attempt)
		return target{repository: repository, integration: integration}, err
	})
	if err != nil {
		return agent.GitCredential{}, err
	}
	host, ok := agent.CodeHostFor(loaded.integration.Provider)
	if !ok {
		return agent.GitCredential{}, fmt.Errorf("no code host registered for %s", loaded.integration.Provider)
	}
	cred, err := host.CloneCredential(ctx, loaded.integration, loaded.repository)
	if err != nil {
		return agent.GitCredential{}, err
	}
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttemptEvent, error) {
		if _, err := l.claimed(tx, attempt.Id); err != nil {
			return nil, err
		}
		return agent.AppendEvent(tx, attempt.Id, agent.EventCredential, map[string]any{"kind": "git", "provider": loaded.integration.Provider, "executor": l.Executor, "expiresAt": cred.ExpiresAt}, time.Now().UTC())
	})
	return cred, err
}

// repositoryOf loads the attempt's repository and its enabled code host
// integration.
func repositoryOf(tx *sql.Tx, attempt *models.AgentAttempt) (*models.Repository, *models.Integration, error) {
	if attempt.RepositoryId == nil {
		return nil, nil, errors.New("the project has no repository bound")
	}
	repository, err := transactional.RepositoryRepository.FindById(tx, *attempt.RepositoryId)
	if err != nil {
		return nil, nil, err
	}
	if repository == nil || repository.IntegrationId == nil {
		return nil, nil, errors.New("the repository has no code host integration")
	}
	integration, err := transactional.IntegrationRepository.FindById(tx, *repository.IntegrationId)
	if err != nil {
		return nil, nil, err
	}
	if integration == nil || !integration.Enabled {
		return nil, nil, errors.New("the repository's integration is gone or disabled")
	}
	return repository, integration, nil
}

func (l Local) Transition(ctx context.Context, attemptId uuid.UUID, to string) error {
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		if _, err := l.claimed(tx, attemptId); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, agent.Transition(tx, attemptId, to, time.Now().UTC())
	})
	if errors.Is(err, agent.ErrIllegalTransition) {
		return fmt.Errorf("%w: %v", ErrClaimLost, err)
	}
	return err
}

func (l Local) Renew(ctx context.Context, attemptId uuid.UUID) (bool, error) {
	return db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return agent.Renew(tx, attemptId, l.ClaimID, time.Now().UTC())
	})
}

// Events appends a batch to the attempt's stream and renews the lease in
// the same transaction, so the heartbeat is the event stream itself.
func (l Local) Events(ctx context.Context, attemptId uuid.UUID, batch []agents.Event) error {
	held, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		if _, err := l.claimed(tx, attemptId); err != nil {
			return false, err
		}
		now := time.Now().UTC()
		for _, event := range batch {
			payload, err := eventPayload(event)
			if err != nil {
				return false, err
			}
			if _, err := agent.AppendEvent(tx, attemptId, event.Kind, payload, now); err != nil {
				return false, err
			}
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	if !held {
		return ErrClaimLost
	}
	return nil
}

func (l Local) Finding(ctx context.Context, attemptId uuid.UUID, body string) error {
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		attempt, err := l.claimed(tx, attemptId)
		if err != nil {
			return struct{}{}, err
		}
		if attempt == nil {
			return struct{}{}, ErrClaimLost
		}
		_, err = agent.Post(tx, attempt, agent.Message{Direction: models.MessageOutbound, Provider: agent.ProviderAgent, Kind: models.MessageKindFinding, Body: body}, nil, "", time.Now().UTC())
		return struct{}{}, err
	})
	return err
}

func (l Local) Blob(ctx context.Context, attemptId uuid.UUID, name string, content []byte, appendTo bool) error {
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		if _, err := l.claimed(tx, attemptId); err != nil {
			return struct{}{}, err
		}
		if storage.Store == nil {
			return struct{}{}, nil
		}
		key := agent.BlobKey(attemptId, name)
		if appendTo {
			existing, err := storage.Store.Read(ctx, key)
			if err != nil && !errors.Is(err, storage.ErrNotFound) {
				return struct{}{}, err
			}
			content = append(existing, content...)
		}
		return struct{}{}, storage.Store.Write(ctx, key, content)
	})
	return err
}

// Finish applies a run's outcome: for a fix it opens the draft pull request
// first, then FinishWithPullRequest records everything.
func (l Local) Finish(ctx context.Context, attemptId uuid.UUID, outcome Outcome) error {
	var pr *agent.Link
	if outcome.Status == agents.StatusFixed {
		attempt, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
			return l.claimed(tx, attemptId)
		})
		if err != nil {
			return err
		}
		if attempt == nil {
			return ErrClaimLost
		}
		branch := outcome.Branch
		if branch == "" {
			branch = attempt.FixBranch
		}
		opened, err := l.openPullRequest(ctx, attempt, branch, outcome.Report)
		if err != nil {
			return err
		}
		pr = &opened
	}
	return l.FinishWithPullRequest(ctx, attemptId, outcome, pr)
}

// FinishWithPullRequest records a run's outcome with a pull request that
// already exists (opened by Finish, or by a CI executor reporting in): for
// a fix the link and the thread message, for a question or an analysis the
// report in the thread, for an error the reason; each ends with the
// matching status. The report is stored as a blob first so the row can
// point at it.
func (l Local) FinishWithPullRequest(ctx context.Context, attemptId uuid.UUID, outcome Outcome, pr *agent.Link) error {
	attempt, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return l.claimed(tx, attemptId)
	})
	if err != nil {
		return err
	}
	if attempt == nil {
		return ErrClaimLost
	}
	if outcome.Status == agents.StatusFixed && pr == nil {
		return errors.New("a fixed outcome needs a pull request")
	}
	branch := outcome.Branch
	if branch == "" {
		branch = attempt.FixBranch
	}
	reportKey := attempt.ReportKey
	if outcome.Report != "" {
		if err := l.Blob(ctx, attemptId, agent.BlobReport, []byte(outcome.Report), false); err != nil {
			return err
		} else if storage.Store != nil {
			reportKey = agent.BlobKey(attemptId, agent.BlobReport)
		}
	}
	row := models.AttemptOutcome{
		Executor:     l.Executor,
		Agent:        attempt.Agent,
		Model:        attempt.Model,
		BaseBranch:   attempt.BaseBranch,
		FixBranch:    branch,
		CostUSD:      outcome.Usage.CostUSD,
		InputTokens:  outcome.Usage.InputTokens,
		OutputTokens: outcome.Usage.OutputTokens,
		Turns:        outcome.Usage.Turns,
		ReportKey:    reportKey,
		Error:        outcome.Error,
	}

	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		current, err := l.claimed(tx, attemptId)
		if err != nil {
			return struct{}{}, err
		}
		row.CostUSD += current.CostUSD
		row.InputTokens += current.InputTokens
		row.OutputTokens += current.OutputTokens
		row.Turns += current.Turns
		now := time.Now().UTC()
		if err := transactional.AgentAttemptRepository.SetOutcome(tx, attemptId, row, now); err != nil {
			return struct{}{}, err
		}
		var message *agent.Message
		to := ""
		switch outcome.Status {
		case agents.StatusFixed:
			if _, err := agent.RecordLink(tx, attemptId, *pr, now); err != nil {
				return struct{}{}, err
			}
			message = &agent.Message{Kind: models.MessageKindPR, Body: "Opened a draft pull request: " + pr.URL}
			to = models.AttemptAwaitingReview
		case agents.StatusQuestion:
			message = &agent.Message{Kind: models.MessageKindQuestion, Body: outcome.Report}
			to = models.AttemptNeedsInput
		case agents.StatusAnalysis:
			message = &agent.Message{Kind: models.MessageKindFinding, Body: outcome.Report}
			to = models.AttemptAnalyzed
		case agents.StatusError:
			to = models.AttemptFailed
		default:
			return struct{}{}, fmt.Errorf("unknown outcome status %q", outcome.Status)
		}
		if message != nil {
			message.Direction = models.MessageOutbound
			message.Provider = agent.ProviderAgent
			if _, err := agent.Post(tx, attempt, *message, nil, "", now); err != nil {
				return struct{}{}, err
			}
		}
		err = agent.Transition(tx, attemptId, to, now)
		return struct{}{}, err
	})
	if errors.Is(err, agent.ErrIllegalTransition) {
		return fmt.Errorf("%w: %v", ErrClaimLost, err)
	}
	if err == nil {
		agent.RecordCost(outcome.Usage.CostUSD)
	}
	return err
}

func (l Local) openPullRequest(ctx context.Context, attempt *models.AgentAttempt, branch string, report string) (agent.Link, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Hold the claim's write lock through publication so cancellation or
	// reclamation cannot authorize a replacement publisher mid-request.
	return db.ExecuteTransaction(func(tx *sql.Tx) (agent.Link, error) {
		attempt, err := l.claimed(tx, attempt.Id)
		if err != nil {
			return agent.Link{}, err
		}
		repository, integration, err := repositoryOf(tx, attempt)
		if err != nil {
			return agent.Link{}, err
		}
		links, err := transactional.AgentLinkRepository.FindByAttempt(tx, attempt.Id)
		if err != nil {
			return agent.Link{}, err
		}
		host, ok := agent.CodeHostFor(integration.Provider)
		if !ok {
			return agent.Link{}, fmt.Errorf("no code host registered for %s", integration.Provider)
		}
		body := report
		for _, link := range links {
			if link.Provider == integration.Provider && link.Kind == models.LinkKindPR && link.IntegrationId != nil && *link.IntegrationId == integration.Id {
				return agent.Link{Provider: link.Provider, Kind: link.Kind, ExternalRef: link.ExternalRef, URL: link.URL, IntegrationId: *link.IntegrationId}, nil
			}
			if link.Provider == integration.Provider && link.Kind == models.LinkKindIssue {
				if _, number, ok := cutRef(link.ExternalRef); ok {
					body = "Fixes #" + number + "\n\n" + body
				}
			}
		}
		opened, err := host.OpenPullRequest(ctx, integration, repository, agent.PullRequest{
			Title: fmt.Sprintf("Fix %s (attempt %d)", attempt.SubjectRef, attempt.Number),
			Body:  body,
			Head:  branch,
			Base:  attempt.BaseBranch,
			Draft: true,
		})
		if err != nil {
			return agent.Link{}, err
		}
		if _, err := agent.RecordLink(tx, attempt.Id, opened, time.Now().UTC()); err != nil {
			return agent.Link{}, err
		}
		return opened, nil
	})
}

func cutRef(ref string) (string, string, bool) {
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == '#' {
			return ref[:i], ref[i+1:], true
		}
	}
	return ref, "", false
}
