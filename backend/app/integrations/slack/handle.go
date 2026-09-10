package slack

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/slack-go/slack"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

const (
	actionFixIt   = "traceway_fix_it"
	actionArchive = "traceway_archive"
	actionView    = "traceway_view"
	actionApprove = "traceway_approve"
	actionDismiss = "traceway_dismiss"
)

var (
	hashRe      = regexp.MustCompile(`^[0-9a-f]{16}$`)
	issuePathRe = regexp.MustCompile(`/issues/([0-9a-f]{16})`)
	mentionRe   = regexp.MustCompile(`<@[A-Z0-9]+>`)
)

const usageText = "Tell me which issue to fix: `fix <issue url>` or `fix <hash>`."

// issueTarget is what the alert message's buttons carry: enough to name
// the subject without trusting anything else in the interaction.
type issueTarget struct {
	ProjectId string `json:"projectId"`
	Hash      string `json:"hash"`
}

// requests turns a mention, a slash command or a button press into attempt
// requests. Anything the user got wrong is answered in Slack and produces
// no request.
func (a *App) requests(ctx context.Context, in *models.Integration, env envelope) ([]agent.Request, error) {
	s, err := a.settings(in)
	if err != nil {
		return nil, err
	}
	api := a.api(s)
	if env.kind == kindAction {
		return a.action(ctx, api, in, env)
	}

	ref, ok := commandRef(env.text)
	if !ok {
		a.reply(ctx, api, env, usageText)
		return nil, nil
	}
	author, err := a.identify(ctx, in, api, env.userId)
	if err != nil {
		return nil, err
	}
	if author.UserId == 0 {
		a.reply(ctx, api, env, a.linkAccountText())
		return nil, nil
	}
	subject, problem, err := a.resolveSubject(ctx, in.OrganizationId, ref)
	if err != nil {
		return nil, err
	}
	if problem == "" {
		problem, err = a.writeProblem(subject.ProjectId, author.UserId)
		if err != nil {
			return nil, err
		}
	}
	if problem != "" {
		a.reply(ctx, api, env, problem)
		return nil, nil
	}
	origin := agent.Link{Provider: Provider, Kind: models.LinkKindOrigin, ExternalRef: threadRef(env.channel, env.ts)}
	if env.ts != "" {
		origin.URL = a.permalink(ctx, api, env.channel, env.ts)
	}
	return []agent.Request{{Subject: subject, RequestedBy: author, Origin: origin}}, nil
}

func (a *App) action(ctx context.Context, api *slack.Client, in *models.Integration, env envelope) ([]agent.Request, error) {
	if env.actionId == actionApprove || env.actionId == actionDismiss {
		return nil, a.decide(ctx, api, in, env)
	}
	if env.actionId != actionFixIt && env.actionId != actionArchive {
		return nil, nil
	}
	var target issueTarget
	if err := json.Unmarshal([]byte(env.actionValue), &target); err != nil {
		return nil, fmt.Errorf("slack: button value: %w", err)
	}
	author, err := a.identify(ctx, in, api, env.userId)
	if err != nil {
		return nil, err
	}
	if author.UserId == 0 {
		a.reply(ctx, api, env, a.linkAccountText())
		return nil, nil
	}
	subject, problem, err := a.subjectFromTarget(in.OrganizationId, target)
	if err != nil {
		return nil, err
	}
	if problem == "" {
		problem, err = a.writeProblem(subject.ProjectId, author.UserId)
		if err != nil {
			return nil, err
		}
	}
	if problem != "" {
		a.reply(ctx, api, env, problem)
		return nil, nil
	}
	if env.actionId == actionArchive {
		if err := telemetry.ExceptionStackTraceRepository.ArchiveByHashes(ctx, subject.ProjectId, []string{subject.Ref}); err != nil {
			return nil, err
		}
		a.reply(ctx, api, env, fmt.Sprintf("Archived by <@%s>.", env.userId))
		return nil, nil
	}
	origin := agent.Link{Provider: Provider, Kind: models.LinkKindAlert, ExternalRef: threadRef(env.channel, env.ts), URL: a.permalink(ctx, api, env.channel, env.ts)}
	return []agent.Request{{Subject: subject, RequestedBy: author, Origin: origin}}, nil
}

// approvalTarget is what the approval message's buttons carry.
type approvalTarget struct {
	AttemptId string `json:"attemptId"`
}

// decide handles the Approve and Dismiss buttons of a pending attempt: the
// member needs write access on the attempt's project, and the answer goes
// back under the message.
func (a *App) decide(ctx context.Context, api *slack.Client, in *models.Integration, env envelope) error {
	var target approvalTarget
	if err := json.Unmarshal([]byte(env.actionValue), &target); err != nil {
		return fmt.Errorf("slack: button value: %w", err)
	}
	attemptId, err := uuid.Parse(target.AttemptId)
	if err != nil {
		return fmt.Errorf("slack: button value: %w", err)
	}
	author, err := a.identify(ctx, in, api, env.userId)
	if err != nil {
		return err
	}
	if author.UserId == 0 {
		a.reply(ctx, api, env, a.linkAccountText())
		return nil
	}
	attempt, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentAttempt, error) {
		return transactional.AgentAttemptRepository.FindById(tx, attemptId)
	})
	if err != nil {
		return err
	}
	if attempt == nil || attempt.OrganizationId != in.OrganizationId {
		a.reply(ctx, api, env, "That attempt no longer exists.")
		return nil
	}
	problem, err := a.writeProblem(attempt.ProjectId, author.UserId)
	if err != nil {
		return err
	}
	if problem != "" {
		a.reply(ctx, api, env, problem)
		return nil
	}
	now := time.Now().UTC()
	if env.actionId == actionApprove {
		approved, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
			return agent.Approve(tx, attempt.Id, author.UserId, now)
		})
		if err != nil {
			return err
		}
		if !approved {
			a.reply(ctx, api, env, "That attempt is not waiting for approval any more.")
			return nil
		}
		agent.Wake()
		a.reply(ctx, api, env, fmt.Sprintf("Approved by <@%s>. The agent is starting.", env.userId))
		return nil
	}
	cancelled, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return agent.Cancel(tx, attempt.Id, author.UserId, now)
	})
	if err != nil {
		return err
	}
	if !cancelled {
		a.reply(ctx, api, env, "That attempt has already finished.")
		return nil
	}
	a.reply(ctx, api, env, fmt.Sprintf("Dismissed by <@%s>.", env.userId))
	return nil
}

// replies turns a message in a thread the app opened into an inbound
// message. Other threads, bots and the thread's own parent are ignored.
func (a *App) replies(ctx context.Context, in *models.Integration, env envelope) ([]agent.InboundMessage, error) {
	if env.fromBot || env.userId == "" || env.threadTs == "" || env.threadTs == env.ts {
		return nil, nil
	}
	thread := agent.Link{Provider: Provider, Kind: models.LinkKindThread, ExternalRef: threadRef(env.channel, env.threadTs)}
	link, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.AgentLink, error) {
		return transactional.AgentLinkRepository.FindInbound(tx, in.Id, Provider, thread.Kind, thread.ExternalRef)
	})
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, nil
	}
	s, err := a.settings(in)
	if err != nil {
		return nil, err
	}
	api := a.api(s)
	author, err := a.identify(ctx, in, api, env.userId)
	if err != nil {
		return nil, err
	}
	if author.UserId == 0 {
		a.reply(ctx, api, env, a.linkAccountText())
		return nil, nil
	}
	return []agent.InboundMessage{{Thread: thread, Author: author, Body: env.text, ExternalRef: threadRef(env.channel, env.ts)}}, nil
}

// commandRef extracts the issue reference from "@Traceway fix <ref>" or
// "/traceway fix <ref>".
func commandRef(text string) (string, bool) {
	words := strings.Fields(mentionRe.ReplaceAllString(text, " "))
	if len(words) != 2 || !strings.EqualFold(words[0], "fix") {
		return "", false
	}
	ref := strings.Trim(words[1], "<>")
	if i := strings.IndexByte(ref, '|'); i >= 0 {
		ref = ref[:i]
	}
	return ref, ref != ""
}

// resolveSubject maps an issue URL or a bare hash onto a project of the
// organization. A bare hash is looked up in every project; the user gets a
// sentence back when that is not conclusive.
func (a *App) resolveSubject(ctx context.Context, organizationId int, ref string) (agent.Subject, string, error) {
	hash, projectId := "", ""
	if parsed, err := url.Parse(ref); err == nil && parsed.Scheme != "" {
		match := issuePathRe.FindStringSubmatch(parsed.Path)
		if match == nil {
			return agent.Subject{}, "That link is not an issue page.", nil
		}
		hash, projectId = match[1], parsed.Query().Get("projectId")
	} else if hashRe.MatchString(ref) {
		hash = ref
	} else {
		return agent.Subject{}, usageText, nil
	}
	if projectId != "" {
		id, err := uuid.Parse(projectId)
		if err != nil {
			return agent.Subject{}, "That link carries an invalid project id.", nil
		}
		return a.subjectFromTarget(organizationId, issueTarget{ProjectId: id.String(), Hash: hash})
	}

	projects, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Project, error) {
		return transactional.ProjectRepository.FindByOrganizationId(tx, organizationId)
	})
	if err != nil {
		return agent.Subject{}, "", err
	}
	var found []uuid.UUID
	for _, project := range projects {
		group, _, _, err := telemetry.ExceptionStackTraceRepository.FindByHash(ctx, project.Id, hash, 1, 1)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return agent.Subject{}, "", err
		}
		if group != nil {
			found = append(found, project.Id)
		}
	}
	switch len(found) {
	case 0:
		return agent.Subject{}, fmt.Sprintf("I can't find issue `%s` in this organization.", hash), nil
	case 1:
		return agent.Subject{Kind: models.SubjectKindTracewayException, Ref: hash, ProjectId: found[0]}, "", nil
	}
	return agent.Subject{}, fmt.Sprintf("Issue `%s` exists in several projects; send the issue link instead.", hash), nil
}

func (a *App) subjectFromTarget(organizationId int, target issueTarget) (agent.Subject, string, error) {
	if !hashRe.MatchString(target.Hash) {
		return agent.Subject{}, "That is not an issue hash.", nil
	}
	projectId, err := uuid.Parse(target.ProjectId)
	if err != nil {
		return agent.Subject{}, "That project id is invalid.", nil
	}
	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return transactional.ProjectRepository.FindById(tx, projectId)
	})
	if err != nil {
		return agent.Subject{}, "", err
	}
	if project == nil || project.OrganizationId == nil || *project.OrganizationId != organizationId {
		return agent.Subject{}, "That issue belongs to another organization.", nil
	}
	return agent.Subject{Kind: models.SubjectKindTracewayException, Ref: target.Hash, ProjectId: projectId}, "", nil
}

// writeProblem is the sentence a member without write access gets back;
// empty when they may start attempts on the project.
func (a *App) writeProblem(projectId uuid.UUID, userId int) (string, error) {
	role, err := db.ExecuteTransaction(func(tx *sql.Tx) (string, error) {
		return transactional.ProjectRepository.GetEffectiveRole(tx, projectId, userId)
	})
	if err != nil {
		return "", err
	}
	switch role {
	case "":
		return "You are not a member of that project's organization.", nil
	case "readonly":
		return "Your role on that project is read-only, so you cannot start attempts.", nil
	}
	return "", nil
}

// identify maps the Slack member onto a Traceway user: a stored identity,
// or the workspace email matched against users on first contact.
func (a *App) identify(ctx context.Context, in *models.Integration, api *slack.Client, slackUserId string) (agent.Identity, error) {
	if slackUserId == "" {
		return agent.Identity{Provider: Provider}, nil
	}
	externalId := fmt.Sprintf("%d:%s", in.Id, slackUserId)
	stored, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Identity, error) {
		return transactional.IdentityRepository.FindByExternalId(tx, Provider, externalId)
	})
	if err != nil {
		return agent.Identity{}, err
	}
	if stored != nil {
		return agent.Identity{UserId: stored.UserId, Provider: Provider, ExternalId: externalId, Display: stored.Display}, nil
	}
	member, err := api.GetUserInfoContext(ctx, slackUserId)
	if err != nil {
		return agent.Identity{}, fmt.Errorf("slack: users.info %s: %w", slackUserId, err)
	}
	display := member.RealName
	if display == "" {
		display = member.Name
	}
	unmapped := agent.Identity{Provider: Provider, ExternalId: externalId, Display: display}
	email := strings.TrimSpace(member.Profile.Email)
	if email == "" {
		return unmapped, nil
	}
	identity, err := db.ExecuteTransaction(func(tx *sql.Tx) (agent.Identity, error) {
		user, err := transactional.UserRepository.FindByEmail(tx, email)
		if err != nil || user == nil {
			return unmapped, err
		}
		row := &models.Identity{UserId: user.Id, Provider: Provider, ExternalId: externalId, Display: display, CreatedAt: time.Now().UTC()}
		if _, err := transactional.IdentityRepository.Create(tx, row); err != nil {
			return unmapped, err
		}
		return agent.Identity{UserId: user.Id, Provider: Provider, ExternalId: externalId, Display: display}, nil
	})
	return identity, err
}

func (a *App) linkAccountText() string {
	base := a.dashboardURL()
	if base == "" {
		return "Your Slack account is not linked to a Traceway user: sign in to Traceway with the email address of this Slack account."
	}
	return fmt.Sprintf("Your Slack account is not linked to a Traceway user: sign in at %s with the email address of this Slack account.", base)
}

// reply answers the person in the thread they wrote in, under the message
// they pressed or mentioned the app in, or privately when there is no
// message to thread under (a slash command).
func (a *App) reply(ctx context.Context, api *slack.Client, env envelope, text string) {
	threadTs := env.threadTs
	if threadTs == "" {
		threadTs = env.ts
	}
	var err error
	switch {
	case env.channel != "" && threadTs != "":
		_, _, err = api.PostMessageContext(ctx, env.channel, slack.MsgOptionTS(threadTs), slack.MsgOptionText(text, false))
	case env.channel != "" && env.userId != "":
		_, err = api.PostEphemeralContext(ctx, env.channel, env.userId, slack.MsgOptionText(text, false))
	default:
		return
	}
	if err != nil {
		traceway.CaptureException(fmt.Errorf("slack: reply in %s: %w", env.channel, err))
	}
}

func (a *App) permalink(ctx context.Context, api *slack.Client, channel string, ts string) string {
	if channel == "" || ts == "" {
		return ""
	}
	link, err := api.GetPermalinkContext(ctx, &slack.PermalinkParameters{Channel: channel, Ts: ts})
	if err != nil {
		return ""
	}
	return link
}
