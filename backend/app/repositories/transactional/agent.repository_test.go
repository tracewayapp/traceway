//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package transactional

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
)

type agentFixture struct {
	orgId     int
	projectId uuid.UUID
	userId    int
}

func beginAgentTx(t *testing.T) (*sql.Tx, agentFixture) {
	t.Helper()
	dbtest.SetupSQLite(t)
	tx, err := db.DB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { tx.Rollback() })
	user, err := UserRepository.Create(tx, "agent@example.com", "Agent Tester", "hashed")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	org, err := OrganizationRepository.Create(tx, "Agent Org", "UTC")
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	project, err := ProjectRepository.CreateWithOrganization(tx, "Agent Project", "opentelemetry", org.Id)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return tx, agentFixture{orgId: org.Id, projectId: project.Id, userId: user.Id}
}

func newAttempt(fx agentFixture, subjectRef string, number int, status string, at time.Time) *models.AgentAttempt {
	return &models.AgentAttempt{
		Id:             uuid.New(),
		OrganizationId: fx.orgId,
		ProjectId:      fx.projectId,
		Number:         number,
		Kind:           models.AttemptKindFix,
		SubjectKind:    models.SubjectKindTracewayException,
		SubjectRef:     subjectRef,
		Status:         status,
		CreatedAt:      at,
		UpdatedAt:      at,
	}
}

func TestAgentAttemptOneActivePerSubject(t *testing.T) {
	tx, fx := beginAgentTx(t)
	now := time.Now().UTC()

	first := newAttempt(fx, "0123456789abcdef", 1, models.AttemptQueued, now)
	if err := AgentAttemptRepository.Create(tx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	number, err := AgentAttemptRepository.NextNumber(tx, fx.projectId, models.SubjectKindTracewayException, "0123456789abcdef")
	if err != nil || number != 2 {
		t.Fatalf("NextNumber = %d, %v", number, err)
	}

	second := newAttempt(fx, "0123456789abcdef", 2, models.AttemptQueued, now)
	if err := AgentAttemptRepository.Create(tx, second); err == nil || !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Fatalf("a second active attempt for the subject must hit the partial unique index, got %v", err)
	}

	active, err := AgentAttemptRepository.FindActiveBySubject(tx, fx.projectId, models.SubjectKindTracewayException, "0123456789abcdef")
	if err != nil || active == nil || active.Id != first.Id {
		t.Fatalf("FindActiveBySubject = %v, %v", active, err)
	}

	if ok, err := AgentAttemptRepository.Transition(tx, first.Id, []string{models.AttemptQueued}, models.AttemptCancelled, now); err != nil || !ok {
		t.Fatalf("cancel: %v %v", ok, err)
	}
	if err := AgentAttemptRepository.Create(tx, second); err != nil {
		t.Fatalf("a new attempt after the first went terminal must be accepted: %v", err)
	}
	if active, _ := AgentAttemptRepository.FindActiveBySubject(tx, fx.projectId, models.SubjectKindTracewayException, "0123456789abcdef"); active == nil || active.Id != second.Id {
		t.Fatalf("active after cancel = %v", active)
	}
	history, err := AgentAttemptRepository.FindBySubject(tx, fx.projectId, models.SubjectKindTracewayException, "0123456789abcdef")
	if err != nil || len(history) != 2 || history[0].Number != 2 {
		t.Fatalf("FindBySubject = %d rows, %v", len(history), err)
	}
}

func TestAgentAttemptClaimLeaseAndReclaim(t *testing.T) {
	tx, fx := beginAgentTx(t)
	now := time.Now().UTC().Truncate(time.Second)

	attempt := newAttempt(fx, "aaaaaaaaaaaaaaaa", 1, models.AttemptQueued, now)
	if err := AgentAttemptRepository.Create(tx, attempt); err != nil {
		t.Fatal(err)
	}
	claimable, err := AgentAttemptRepository.FindClaimable(tx, 10)
	if err != nil || len(claimable) != 1 {
		t.Fatalf("FindClaimable = %d, %v", len(claimable), err)
	}

	lease := now.Add(2 * time.Minute)
	if ok, err := AgentAttemptRepository.Claim(tx, attempt.Id, "embedded-1", lease, now); err != nil || !ok {
		t.Fatalf("claim: %v %v", ok, err)
	}
	if ok, _ := AgentAttemptRepository.Claim(tx, attempt.Id, "embedded-2", lease, now); ok {
		t.Fatal("a second claimer must lose")
	}
	claimed, _ := AgentAttemptRepository.FindById(tx, attempt.Id)
	if claimed.Status != models.AttemptClaimed || claimed.Executor != "embedded-1" || claimed.ClaimedBy == "embedded-1" || claimed.ClaimedBy == "" || claimed.LeaseExpiresAt == nil || !claimed.LeaseExpiresAt.Equal(lease) || claimed.StartedAt == nil {
		t.Fatalf("claimed row = %+v", claimed)
	}

	if ok, err := AgentAttemptRepository.Transition(tx, attempt.Id, []string{models.AttemptClaimed}, models.AttemptRunning, now); err != nil || !ok {
		t.Fatalf("claimed -> running: %v %v", ok, err)
	}
	if ok, _ := AgentAttemptRepository.RenewLease(tx, attempt.Id, "embedded-2", lease.Add(time.Minute), now); ok {
		t.Fatal("renewing someone else's lease must fail")
	}
	if ok, err := AgentAttemptRepository.RenewLease(tx, attempt.Id, claimed.ClaimedBy, lease.Add(time.Minute), now); err != nil || !ok {
		t.Fatalf("renew: %v %v", ok, err)
	}

	if n, err := AgentAttemptRepository.ReclaimStale(tx, lease); err != nil || n != 0 {
		t.Fatalf("reclaim before expiry = %d, %v", n, err)
	}
	if n, err := AgentAttemptRepository.ReclaimStale(tx, lease.Add(2*time.Minute)); err != nil || n != 1 {
		t.Fatalf("reclaim after expiry = %d, %v", n, err)
	}
	reclaimed, _ := AgentAttemptRepository.FindById(tx, attempt.Id)
	if reclaimed.Status != models.AttemptQueued || !reclaimed.Resume || reclaimed.ClaimedBy != "" || reclaimed.LeaseExpiresAt != nil {
		t.Fatalf("reclaimed row = %+v", reclaimed)
	}
}

func TestAgentAttemptTransitionsAreGuarded(t *testing.T) {
	tx, fx := beginAgentTx(t)
	now := time.Now().UTC()
	attempt := newAttempt(fx, "bbbbbbbbbbbbbbbb", 1, models.AttemptPendingApproval, now)
	if err := AgentAttemptRepository.Create(tx, attempt); err != nil {
		t.Fatal(err)
	}

	if ok, _ := AgentAttemptRepository.Transition(tx, attempt.Id, []string{models.AttemptRunning}, models.AttemptVerifying, now); ok {
		t.Fatal("a transition from a status the row is not in must return no rows")
	}
	if ok, err := AgentAttemptRepository.Approve(tx, attempt.Id, fx.userId, now); err != nil || !ok {
		t.Fatalf("approve: %v %v", ok, err)
	}
	if ok, _ := AgentAttemptRepository.Approve(tx, attempt.Id, fx.userId, now); ok {
		t.Fatal("approving twice must fail")
	}
	approved, _ := AgentAttemptRepository.FindById(tx, attempt.Id)
	if approved.Status != models.AttemptQueued || approved.ApprovedBy == nil || *approved.ApprovedBy != fx.userId {
		t.Fatalf("approved row = %+v", approved)
	}

	if _, err := AgentAttemptRepository.Transition(tx, attempt.Id, nil, models.AttemptFailed, now); err == nil {
		t.Fatal("a transition without source statuses must be refused")
	}
	if ok, err := AgentAttemptRepository.Transition(tx, attempt.Id, []string{models.AttemptQueued, models.AttemptClaimed}, models.AttemptFailed, now); err != nil || !ok {
		t.Fatalf("queued -> failed: %v %v", ok, err)
	}
	failed, _ := AgentAttemptRepository.FindById(tx, attempt.Id)
	if failed.FinishedAt == nil || failed.Status != models.AttemptFailed {
		t.Fatalf("terminal transition must stamp finished_at: %+v", failed)
	}

	if err := AgentAttemptRepository.SetOutcome(tx, attempt.Id, models.AttemptOutcome{Executor: "embedded", Agent: "claude-code", Model: "m", FixBranch: "traceway/fix-1", CostUSD: 1.25, InputTokens: 10, OutputTokens: 20, Turns: 3, ReportKey: "agent/x/report.md", Error: "boom"}, now); err != nil {
		t.Fatal(err)
	}
	withOutcome, _ := AgentAttemptRepository.FindById(tx, attempt.Id)
	if withOutcome.CostUSD != 1.25 || withOutcome.Turns != 3 || withOutcome.FixBranch != "traceway/fix-1" || withOutcome.Error != "boom" || withOutcome.Status != models.AttemptFailed {
		t.Fatalf("outcome row = %+v", withOutcome)
	}

	counts, err := AgentAttemptRepository.CountByStatus(tx)
	if err != nil || len(counts) != 1 || counts[0].Status != models.AttemptFailed || counts[0].Count != 1 {
		t.Fatalf("CountByStatus = %+v, %v", counts, err)
	}
	page, err := AgentAttemptRepository.FindByProject(tx, fx.projectId, "", 10, 0)
	if err != nil || len(page) != 1 {
		t.Fatalf("FindByProject = %d, %v", len(page), err)
	}
	if n, _ := AgentAttemptRepository.CountByProject(tx, fx.projectId, models.AttemptQueued); n != 0 {
		t.Fatalf("CountByProject(queued) = %d", n)
	}
}

func TestAgentAttemptEventsSequenceStrictly(t *testing.T) {
	tx, fx := beginAgentTx(t)
	now := time.Now().UTC()
	attempt := newAttempt(fx, "cccccccccccccccc", 1, models.AttemptRunning, now)
	if err := AgentAttemptRepository.Create(tx, attempt); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		event, err := AgentAttemptEventRepository.Append(tx, attempt.Id, "progress", models.JSONText(`{"n":`+string(rune('0'+i))+`}`), now)
		if err != nil || event.Seq != i {
			t.Fatalf("append %d: seq=%v err=%v", i, event, err)
		}
	}
	after, err := AgentAttemptEventRepository.ListAfter(tx, attempt.Id, 2, 10)
	if err != nil || len(after) != 3 || after[0].Seq != 3 || after[2].Seq != 5 {
		t.Fatalf("ListAfter(2) = %+v, %v", after, err)
	}
	if _, err := AgentAttemptEventRepository.Append(tx, attempt.Id, "progress", nil, now); err != nil {
		t.Fatalf("append with an empty payload: %v", err)
	}
	if latest, _ := AgentAttemptEventRepository.LatestSeq(tx, attempt.Id); latest != 6 {
		t.Fatalf("LatestSeq = %d", latest)
	}

	duplicate := &models.AgentAttemptEvent{AttemptId: attempt.Id, Seq: 6, Kind: "progress", Payload: models.JSONText(`{}`), CreatedAt: now}
	if _, err := insertRawEvent(tx, duplicate); err == nil {
		t.Fatal("a duplicate (attempt_id, seq) must be rejected by the unique index")
	}
	if n, err := AgentAttemptEventRepository.DeleteByAttempt(tx, attempt.Id); err != nil || n != 6 {
		t.Fatalf("DeleteByAttempt = %d, %v", n, err)
	}
}

func TestAgentLinksAndMessages(t *testing.T) {
	tx, fx := beginAgentTx(t)
	now := time.Now().UTC()
	attempt := newAttempt(fx, "dddddddddddddddd", 1, models.AttemptRunning, now)
	if err := AgentAttemptRepository.Create(tx, attempt); err != nil {
		t.Fatal(err)
	}

	link := &models.AgentLink{AttemptId: attempt.Id, Provider: "github", Kind: models.LinkKindPR, ExternalRef: "acme/app#42", URL: "https://github.com/acme/app/pull/42", CreatedAt: now}
	linkId, err := AgentLinkRepository.Create(tx, link)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AgentLinkRepository.Create(tx, &models.AgentLink{AttemptId: attempt.Id, Provider: "github", Kind: models.LinkKindPR, ExternalRef: "acme/app#42", CreatedAt: now}); err == nil {
		t.Fatal("the same artifact linked twice must be rejected")
	}
	found, err := AgentLinkRepository.FindByExternalRef(tx, "github", models.LinkKindPR, "acme/app#42")
	if err != nil || found == nil || found.AttemptId != attempt.Id {
		t.Fatalf("FindByExternalRef = %v, %v", found, err)
	}
	if byKind, _ := AgentLinkRepository.FindByAttemptAndKind(tx, attempt.Id, "github", models.LinkKindPR); byKind == nil || byKind.Id != linkId {
		t.Fatalf("FindByAttemptAndKind = %v", byKind)
	}

	identityId, err := IdentityRepository.Create(tx, &models.Identity{UserId: fx.userId, Provider: "slack", ExternalId: "U123", Display: "@agent", CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := IdentityRepository.Create(tx, &models.Identity{UserId: fx.userId, Provider: "slack", ExternalId: "U123", CreatedAt: now}); err == nil {
		t.Fatal("the same external account mapped twice must be rejected")
	}
	if identity, _ := IdentityRepository.FindByExternalId(tx, "slack", "U123"); identity == nil || identity.UserId != fx.userId {
		t.Fatalf("FindByExternalId = %v", identity)
	}

	for i, body := range []string{"progress one", "a question?", "an answer"} {
		direction, kind := models.MessageOutbound, models.MessageKindProgress
		if i == 2 {
			direction, kind = models.MessageInbound, models.MessageKindAnswer
		}
		msg := &models.AgentMessage{AttemptId: attempt.Id, Direction: direction, Provider: "web", Kind: kind, Body: body, CreatedAt: now.Add(time.Duration(i) * time.Second)}
		if i == 2 {
			msg.LinkId = &linkId
			msg.IdentityId = &identityId
			msg.ExternalRef = "ts-1"
		}
		if _, err := AgentMessageRepository.Create(tx, msg); err != nil {
			t.Fatalf("create message %d: %v", i, err)
		}
	}
	all, err := AgentMessageRepository.ListAfter(tx, attempt.Id, 0, "", 10)
	if err != nil || len(all) != 3 || all[0].Body != "progress one" {
		t.Fatalf("ListAfter = %+v, %v", all, err)
	}
	inbound, _ := AgentMessageRepository.ListAfter(tx, attempt.Id, 0, models.MessageInbound, 10)
	if len(inbound) != 1 || inbound[0].IdentityId == nil || *inbound[0].IdentityId != identityId {
		t.Fatalf("inbound = %+v", inbound)
	}
	if dup, _ := AgentMessageRepository.FindByExternalRef(tx, attempt.Id, "web", "ts-1"); dup == nil || dup.Body != "an answer" {
		t.Fatalf("FindByExternalRef = %v", dup)
	}
	if err := AgentMessageRepository.MarkDelivered(tx, all[0].Id, now); err != nil {
		t.Fatal(err)
	}
	if delivered, _ := AgentMessageRepository.FindById(tx, all[0].Id); delivered.DeliveredAt == nil {
		t.Fatal("MarkDelivered did not stamp delivered_at")
	}
}

func TestAgentProfilesIntegrationsRepositoriesAndSources(t *testing.T) {
	tx, fx := beginAgentTx(t)
	now := time.Now().UTC()

	first := &models.AgentProfile{OrganizationId: fx.orgId, Name: "Claude", Agent: "claude-code", Model: "claude-opus-5", AllowedTools: models.StringSlice{"Read", "Edit"}, NetworkPolicy: models.JSONText(`{}`), IsDefault: true, CreatedAt: now, UpdatedAt: now}
	firstId, err := AgentProfileRepository.Create(tx, first)
	if err != nil {
		t.Fatal(err)
	}
	second := &models.AgentProfile{OrganizationId: fx.orgId, Name: "Codex", Agent: "codex", NetworkPolicy: models.JSONText(`{}`), CreatedAt: now, UpdatedAt: now}
	secondId, err := AgentProfileRepository.Create(tx, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AgentProfileRepository.Create(tx, &models.AgentProfile{OrganizationId: fx.orgId, Name: "Claude", Agent: "claude-code", NetworkPolicy: models.JSONText(`{}`), CreatedAt: now, UpdatedAt: now}); err == nil {
		t.Fatal("duplicate profile name in one org must be rejected")
	}
	if err := AgentProfileRepository.SetDefault(tx, fx.orgId, secondId, now); err != nil {
		t.Fatal(err)
	}
	if def, _ := AgentProfileRepository.FindDefault(tx, fx.orgId); def == nil || def.Id != secondId {
		t.Fatalf("FindDefault = %v", def)
	}
	if old, _ := AgentProfileRepository.FindById(tx, firstId); old.IsDefault {
		t.Fatal("SetDefault must clear the previous default")
	}
	if profiles, _ := AgentProfileRepository.FindByOrganization(tx, fx.orgId); len(profiles) != 2 || profiles[0].Id != secondId || len(profiles[1].AllowedTools) != 2 {
		t.Fatalf("FindByOrganization = %+v", profiles)
	}

	integration := &models.Integration{OrganizationId: fx.orgId, Provider: "github", Kinds: models.StringSlice{"code_host"}, Name: "acme", Config: models.JSONText(`{"token":"v1:x"}`), Enabled: true, CreatedAt: now, UpdatedAt: now}
	integrationId, err := IntegrationRepository.Create(tx, integration)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := IntegrationRepository.Create(tx, &models.Integration{OrganizationId: fx.orgId, Provider: "github", Name: "acme", Config: models.JSONText(`{}`), CreatedAt: now, UpdatedAt: now}); err == nil {
		t.Fatal("duplicate (org, provider, name) must be rejected")
	}
	enabled, err := IntegrationRepository.FindEnabledByProvider(tx, fx.orgId, "github")
	if err != nil || len(enabled) != 1 || string(enabled[0].Config) != `{"token":"v1:x"}` {
		t.Fatalf("FindEnabledByProvider = %+v, %v", enabled, err)
	}

	repo := &models.Repository{ProjectId: fx.projectId, IntegrationId: &integrationId, Owner: "acme", Name: "app", DefaultBranch: "main", CreatedAt: now, UpdatedAt: now}
	if _, err := RepositoryRepository.Create(tx, repo); err != nil {
		t.Fatal(err)
	}
	if _, err := RepositoryRepository.Create(tx, &models.Repository{ProjectId: fx.projectId, Owner: "acme", Name: "other", CreatedAt: now, UpdatedAt: now}); err == nil {
		t.Fatal("a project can bind one repository only")
	}
	bound, err := RepositoryRepository.FindByProject(tx, fx.projectId)
	if err != nil || bound == nil || bound.Name != "app" || bound.IntegrationId == nil {
		t.Fatalf("FindByProject = %+v, %v", bound, err)
	}

	if err := ProjectTelemetrySourceRepository.Replace(tx, fx.projectId, "logs", []int{integrationId}); err != nil {
		t.Fatal(err)
	}
	if err := ProjectTelemetrySourceRepository.Replace(tx, fx.projectId, "logs", []int{integrationId}); err != nil {
		t.Fatalf("Replace must be idempotent: %v", err)
	}
	sources, err := ProjectTelemetrySourceRepository.FindByProject(tx, fx.projectId)
	if err != nil || len(sources) != 1 || sources[0].Domain != "logs" || sources[0].Priority != 0 {
		t.Fatalf("FindByProject = %+v, %v", sources, err)
	}

	runner, err := AgentRunnerRepository.UpsertSeen(tx, "runner-a", "1.0", models.JSONText(`{"agents":["claude-code"]}`), now)
	if err != nil || runner.Id == 0 {
		t.Fatalf("UpsertSeen = %v, %v", runner, err)
	}
	again, err := AgentRunnerRepository.UpsertSeen(tx, "runner-a", "", nil, now.Add(time.Minute))
	if err != nil || again.Id != runner.Id || again.Version != "1.0" || string(again.Capabilities) != `{"agents":["claude-code"]}` {
		t.Fatalf("second UpsertSeen = %+v, %v", again, err)
	}
	if online, _ := AgentRunnerRepository.CountOnline(tx, now.Add(30*time.Second)); online != 1 {
		t.Fatalf("CountOnline = %d", online)
	}
}

func insertRawEvent(tx *sql.Tx, event *models.AgentAttemptEvent) (int, error) {
	return lit.Insert[models.AgentAttemptEvent](tx, event)
}
