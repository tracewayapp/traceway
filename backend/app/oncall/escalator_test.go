//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package oncall

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/notifications"
	"github.com/tracewayapp/traceway/backend/app/outbox"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services"
)

type escalatorFixture struct {
	OrgId     int
	ProjectId uuid.UUID
	Alice     int
	Bob       int
}

func setupEscalatorDB(t *testing.T) *escalatorFixture {
	t.Helper()

	dbtest.SetupSQLite(t)
	services.InitEmail()
	outbox.RegisterSender(notifications.AdapterSend)

	fixture := &escalatorFixture{}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		org, err := transactional.OrganizationRepository.Create(tx, "Acme", "UTC")
		if err != nil {
			return struct{}{}, err
		}
		fixture.OrgId = org.Id
		alice, err := transactional.UserRepository.Create(tx, "alice@example.com", "Alice", "x")
		if err != nil {
			return struct{}{}, err
		}
		bob, err := transactional.UserRepository.Create(tx, "bob@example.com", "Bob", "x")
		if err != nil {
			return struct{}{}, err
		}
		fixture.Alice = alice.Id
		fixture.Bob = bob.Id
		for _, userId := range []int{alice.Id, bob.Id} {
			if _, err := transactional.OrganizationRepository.AddUser(tx, org.Id, userId, "user"); err != nil {
				return struct{}{}, err
			}
		}
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "api", "gin", org.Id)
		if err != nil {
			return struct{}{}, err
		}
		fixture.ProjectId = project.Id
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("seed fixture: %v", err)
	}
	return fixture
}

func createPolicy(t *testing.T, orgId int, definition string) int {
	t.Helper()
	id, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		now := time.Now().UTC()
		return transactional.EscalationPolicyRepository.Create(tx, &models.EscalationPolicy{
			OrganizationId: orgId,
			Name:           fmt.Sprintf("policy-%d", now.UnixNano()),
			Definition:     models.JSONText(definition),
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	return id
}

func openTestPageForPolicy(t *testing.T, fixture *escalatorFixture, policyId int, dedupKey string) *models.Page {
	t.Helper()
	if _, err := openPage(openPageParams{
		PolicyId:  policyId,
		ProjectId: fixture.ProjectId,
		RuleName:  "Test rule",
		RuleType:  "new_error",
		Subject:   "Something broke",
		Body:      "It really broke",
		URL:       "/issues/abc",
		Severity:  "critical",
		DedupKey:  dedupKey,
	}); err != nil {
		t.Fatalf("open page: %v", err)
	}
	page := findPageByDedupKey(t, dedupKey)
	if page == nil {
		t.Fatal("expected a page to exist")
	}
	return page
}

func findPageByDedupKey(t *testing.T, dedupKey string) *models.Page {
	t.Helper()
	page, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Page, error) {
		return transactional.PageRepository.FindUnresolvedByDedupKey(tx, dedupKey)
	})
	if err != nil {
		t.Fatalf("find page: %v", err)
	}
	return page
}

func reloadPage(t *testing.T, id int) *models.Page {
	t.Helper()
	page, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Page, error) {
		return transactional.PageRepository.FindById(tx, id)
	})
	if err != nil || page == nil {
		t.Fatalf("reload page %d: %v", id, err)
	}
	return page
}

func pageNotifications(t *testing.T, pageId int) []*models.PageNotification {
	t.Helper()
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.PageNotification, error) {
		return transactional.PageNotificationRepository.FindByPage(tx, pageId)
	})
	if err != nil {
		t.Fatalf("load notifications: %v", err)
	}
	return rows
}

// tickAndDrain runs one escalator tick followed by one outbox drain tick, so
// enqueued deliveries actually send (via the log-only email adapter). The
// drain runs slightly ahead of the tick time because Enqueue stamps rows with
// its own wall clock; in production Wake() drains with a fresh timestamp.
func tickAndDrain(t *testing.T, now time.Time) {
	t.Helper()
	runEscalatorTick(context.Background(), now)
	drainAt := now.Add(2 * time.Second)
	if realNow := time.Now().UTC().Add(2 * time.Second); realNow.After(drainAt) {
		drainAt = realNow
	}
	outbox.DrainOnce(context.Background(), drainAt)
}

func twoStepPolicy(alice, bob int) string {
	return fmt.Sprintf(`{"schemaVersion":1,"steps":[{"targets":[{"type":"user","id":%d}],"delayMinutes":5},{"targets":[{"type":"user","id":%d}],"delayMinutes":5}],"repeatCount":0}`, alice, bob)
}

func TestEscalatorNotifiesFirstLevelImmediately(t *testing.T) {
	fixture := setupEscalatorDB(t)
	policyId := createPolicy(t, fixture.OrgId, twoStepPolicy(fixture.Alice, fixture.Bob))
	page := openTestPageForPolicy(t, fixture, policyId, "rule1|/issues/abc")

	now := time.Now().UTC()
	tickAndDrain(t, now)

	page = reloadPage(t, page.Id)
	if page.EscalationLevel != 0 {
		t.Errorf("escalation level = %d, want 0", page.EscalationLevel)
	}
	if page.NextEscalationAt == nil {
		t.Fatal("expected a next escalation time")
	}
	if diff := page.NextEscalationAt.Sub(now); diff < 4*time.Minute || diff > 6*time.Minute {
		t.Errorf("next escalation in %v, want ~5m", diff)
	}
	rows := pageNotifications(t, page.Id)
	if len(rows) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(rows))
	}
	if rows[0].Status != models.PageNotificationSent {
		t.Errorf("notification status = %s, want sent (error: %s)", rows[0].Status, rows[0].ErrorMsg)
	}
	if rows[0].UserId == nil || *rows[0].UserId != fixture.Alice {
		t.Errorf("notified user = %v, want alice (%d)", rows[0].UserId, fixture.Alice)
	}
	if rows[0].MethodType != "email" {
		t.Errorf("fallback method = %s, want email", rows[0].MethodType)
	}
}

func TestEscalatorAdvancesLevelsAndExhausts(t *testing.T) {
	fixture := setupEscalatorDB(t)
	policyId := createPolicy(t, fixture.OrgId, twoStepPolicy(fixture.Alice, fixture.Bob))
	page := openTestPageForPolicy(t, fixture, policyId, "rule2|/issues/abc")

	now := time.Now().UTC()
	tickAndDrain(t, now)

	// Not due yet: nothing should change.
	tickAndDrain(t, now.Add(2*time.Minute))
	if rows := pageNotifications(t, page.Id); len(rows) != 1 {
		t.Fatalf("expected still 1 notification before the delay elapses, got %d", len(rows))
	}

	// Due: escalate to L2 (bob).
	tickAndDrain(t, now.Add(6*time.Minute))
	page = reloadPage(t, page.Id)
	if page.EscalationLevel != 1 {
		t.Errorf("escalation level = %d, want 1", page.EscalationLevel)
	}
	rows := pageNotifications(t, page.Id)
	if len(rows) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(rows))
	}
	if rows[1].UserId == nil || *rows[1].UserId != fixture.Bob {
		t.Errorf("second notification user = %v, want bob (%d)", rows[1].UserId, fixture.Bob)
	}

	// Last step with repeatCount 0: exhausted right away, stays open,
	// nothing further on later ticks.
	if page.NextEscalationAt != nil {
		t.Errorf("expected escalation exhausted (next = nil), got %v", page.NextEscalationAt)
	}
	tickAndDrain(t, now.Add(12*time.Minute))
	page = reloadPage(t, page.Id)
	if page.Status != models.PageStatusOpen {
		t.Errorf("page status = %s, want open after exhaustion", page.Status)
	}
	if rows := pageNotifications(t, page.Id); len(rows) != 2 {
		t.Errorf("expected no further notifications after exhaustion, got %d", len(rows))
	}
}

func TestEscalatorRepeatCyclesThenStops(t *testing.T) {
	fixture := setupEscalatorDB(t)
	definition := fmt.Sprintf(`{"schemaVersion":1,"steps":[{"targets":[{"type":"user","id":%d}],"delayMinutes":5}],"repeatCount":1}`, fixture.Alice)
	policyId := createPolicy(t, fixture.OrgId, definition)
	page := openTestPageForPolicy(t, fixture, policyId, "rule3|/issues/abc")

	now := time.Now().UTC()
	tickAndDrain(t, now)                     // iteration 0, level 0
	tickAndDrain(t, now.Add(6*time.Minute))  // repeat: iteration 1, level 0
	tickAndDrain(t, now.Add(12*time.Minute)) // exhausted

	page = reloadPage(t, page.Id)
	if page.RepeatIteration != 1 {
		t.Errorf("repeat iteration = %d, want 1", page.RepeatIteration)
	}
	if page.NextEscalationAt != nil {
		t.Errorf("expected exhaustion, next escalation = %v", page.NextEscalationAt)
	}
	if rows := pageNotifications(t, page.Id); len(rows) != 2 {
		t.Errorf("expected 2 notifications (initial + one repeat), got %d", len(rows))
	}
}

func TestAcknowledgeStopsEscalation(t *testing.T) {
	fixture := setupEscalatorDB(t)
	policyId := createPolicy(t, fixture.OrgId, twoStepPolicy(fixture.Alice, fixture.Bob))
	page := openTestPageForPolicy(t, fixture, policyId, "rule4|/issues/abc")

	now := time.Now().UTC()
	tickAndDrain(t, now)

	acknowledged, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return AcknowledgePage(tx, page.Id, &fixture.Alice, AckViaDashboard, now)
	})
	if err != nil || !acknowledged {
		t.Fatalf("acknowledge failed: %v (ok=%v)", err, acknowledged)
	}

	tickAndDrain(t, now.Add(10*time.Minute))
	if rows := pageNotifications(t, page.Id); len(rows) != 1 {
		t.Errorf("expected no escalation after ack, got %d notifications", len(rows))
	}

	// Second ack loses the guarded update.
	acknowledged, err = db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return AcknowledgePage(tx, page.Id, &fixture.Bob, AckViaDashboard, now)
	})
	if err != nil {
		t.Fatalf("second acknowledge errored: %v", err)
	}
	if acknowledged {
		t.Error("second acknowledge should report no rows affected")
	}
}

func TestAcknowledgeCancelsQueuedDeliveries(t *testing.T) {
	fixture := setupEscalatorDB(t)
	policyId := createPolicy(t, fixture.OrgId, twoStepPolicy(fixture.Alice, fixture.Bob))
	page := openTestPageForPolicy(t, fixture, policyId, "rule9|/issues/abc")

	// Tick WITHOUT draining: the delivery sits queued in the outbox.
	now := time.Now().UTC()
	runEscalatorTick(context.Background(), now)

	acknowledged, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return AcknowledgePage(tx, page.Id, &fixture.Alice, AckViaDashboard, now)
	})
	if err != nil || !acknowledged {
		t.Fatalf("acknowledge failed: %v (ok=%v)", err, acknowledged)
	}

	rows := pageNotifications(t, page.Id)
	if len(rows) != 1 || rows[0].Status != models.PageNotificationCancelled {
		t.Fatalf("expected the queued delivery-log row cancelled, got %+v", rows)
	}

	// A later drain must not deliver the cancelled row.
	outbox.DrainOnce(context.Background(), time.Now().UTC().Add(time.Minute))
	if rows := pageNotifications(t, page.Id); rows[0].Status != models.PageNotificationCancelled {
		t.Errorf("cancelled delivery resurrected to %s", rows[0].Status)
	}
}

func TestClaimRollsBackWhenPageAcknowledgedMidClaim(t *testing.T) {
	fixture := setupEscalatorDB(t)
	policyId := createPolicy(t, fixture.OrgId, twoStepPolicy(fixture.Alice, fixture.Bob))
	page := openTestPageForPolicy(t, fixture, policyId, "race1|/issues/abc")

	now := time.Now().UTC()
	acknowledged, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return AcknowledgePage(tx, page.Id, &fixture.Alice, AckViaDashboard, now)
	})
	if err != nil || !acknowledged {
		t.Fatalf("acknowledge: %v (%v)", err, acknowledged)
	}

	// Simulate the race: a claim that read the page while it was still open
	// (the stale pre-ack row) commits after the ack. The guarded terminal
	// update must lose and roll the whole claim back, so none of its
	// deliveries — which the ack's CancelByKey never saw — can commit.
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return claimPageEscalation(tx, page, now)
	})
	if !errors.Is(err, errClaimLost) {
		t.Fatalf("expected errClaimLost, got %v", err)
	}
	if rows := pageNotifications(t, page.Id); len(rows) != 0 {
		t.Errorf("claim leaked %d page_notifications rows past the ack", len(rows))
	}
	if rows := outboxRowsForPage(t, page.Id); len(rows) != 0 {
		t.Errorf("claim leaked %d outbox rows past the ack", len(rows))
	}
}

func TestPageDedupBumpsAndReleasesOnResolve(t *testing.T) {
	fixture := setupEscalatorDB(t)
	policyId := createPolicy(t, fixture.OrgId, twoStepPolicy(fixture.Alice, fixture.Bob))
	page := openTestPageForPolicy(t, fixture, policyId, "rule5|/issues/abc")

	now := time.Now().UTC()
	tickAndDrain(t, now)
	levelBefore := reloadPage(t, page.Id).EscalationLevel

	// Refire while unresolved: bump only, clock untouched, no re-notify.
	openTestPageForPolicy(t, fixture, policyId, "rule5|/issues/abc")
	bumped := reloadPage(t, page.Id)
	if bumped.EventCount != 2 {
		t.Errorf("event count = %d, want 2", bumped.EventCount)
	}
	if bumped.EscalationLevel != levelBefore {
		t.Errorf("escalation level changed on refire: %d -> %d", levelBefore, bumped.EscalationLevel)
	}

	resolved, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return transactional.PageRepository.Resolve(tx, page.Id, fixture.Alice, now)
	})
	if err != nil || !resolved {
		t.Fatalf("resolve failed: %v (ok=%v)", err, resolved)
	}

	// Same dedup key now opens a fresh page.
	fresh := openTestPageForPolicy(t, fixture, policyId, "rule5|/issues/abc")
	if fresh.Id == page.Id {
		t.Error("expected a fresh page after resolve")
	}
	if fresh.EventCount != 1 {
		t.Errorf("fresh page event count = %d, want 1", fresh.EventCount)
	}
}

// A realistic incident storm: one noisy rule firing 200 times while ticks run
// far more often than the escalation delays. The delivery count must be driven
// by the policy (2 steps, 1 repeat = 4 notifications), never by how many events
// arrived or how often the worker polled.
func TestIncidentStormStaysBoundedByThePolicy(t *testing.T) {
	fixture := setupEscalatorDB(t)
	definition := fmt.Sprintf(
		`{"schemaVersion":1,"steps":[{"targets":[{"type":"user","id":%d}],"delayMinutes":5},{"targets":[{"type":"user","id":%d}],"delayMinutes":5}],"repeatCount":1}`,
		fixture.Alice, fixture.Bob,
	)
	policyId := createPolicy(t, fixture.OrgId, definition)
	page := openTestPageForPolicy(t, fixture, policyId, "storm|/issues/abc")

	start := time.Now().UTC()
	// 30 minutes of 30-second ticks, with the rule refiring on every tick.
	for minute := 0; minute < 30; minute++ {
		for half := 0; half < 2; half++ {
			openTestPageForPolicy(t, fixture, policyId, "storm|/issues/abc")
			tickAndDrain(t, start.Add(time.Duration(minute)*time.Minute+time.Duration(half*30)*time.Second))
		}
	}

	stormed := reloadPage(t, page.Id)
	if stormed.EventCount != 61 {
		t.Errorf("event count = %d, want 61 (1 open + 60 refires)", stormed.EventCount)
	}
	rows := pageNotifications(t, page.Id)
	// L1 alice, L2 bob, then repeat iteration 1: L1 alice, L2 bob.
	if len(rows) != 4 {
		t.Fatalf("60 refires and 60 ticks produced %d notifications, want exactly 4", len(rows))
	}
	for _, row := range rows {
		if row.Status != models.PageNotificationSent {
			t.Errorf("notification %d status = %s, want sent (%s)", row.Id, row.Status, row.ErrorMsg)
		}
	}
	if stormed.NextEscalationAt != nil {
		t.Errorf("escalation should be exhausted, next = %v", stormed.NextEscalationAt)
	}
}

// An override exists to relieve someone. If the escalator paged every user the
// schedule stack covers, the person who handed off their shift would be paged
// alongside the person covering it.
func TestOverridePagesOnlyTheCoveringUser(t *testing.T) {
	fixture := setupEscalatorDB(t)
	now := time.Now().UTC()
	definition := fmt.Sprintf(
		`{"schemaVersion":1,"layers":[{"id":"l1","name":"Base","rotationType":"daily","handoffTime":"09:00","rotationStart":"2020-01-01","userIds":[%d]}]}`,
		fixture.Alice,
	)
	scheduleId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		teamId, err := transactional.TeamRepository.Create(tx, &models.Team{
			OrganizationId: fixture.OrgId, Name: "override-team", CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return 0, err
		}
		id, err := transactional.OncallScheduleRepository.Create(tx, &models.OncallSchedule{
			OrganizationId: fixture.OrgId, TeamId: teamId, Name: "override-sched", Timezone: "UTC",
			Definition: models.JSONText(definition), CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return 0, err
		}
		_, err = transactional.OncallOverrideRepository.Create(tx, &models.OncallOverride{
			ScheduleId: id, UserId: fixture.Bob,
			StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour), CreatedAt: now,
		})
		return id, err
	})
	if err != nil {
		t.Fatalf("seed schedule: %v", err)
	}

	policyDefinition := fmt.Sprintf(`{"schemaVersion":1,"steps":[{"targets":[{"type":"schedule","id":%d}],"delayMinutes":5}],"repeatCount":0}`, scheduleId)
	policyId := createPolicy(t, fixture.OrgId, policyDefinition)
	page := openTestPageForPolicy(t, fixture, policyId, "override|/issues/abc")
	runEscalatorTick(context.Background(), time.Now().UTC())

	rows := pageNotifications(t, page.Id)
	if len(rows) != 1 {
		t.Fatalf("expected only the covering user paged, got %d deliveries: %+v", len(rows), rows)
	}
	if rows[0].UserId == nil || *rows[0].UserId != fixture.Bob {
		t.Errorf("paged user = %v, want bob (%d) who holds the override", rows[0].UserId, fixture.Bob)
	}
}

func TestEscalatorSkipsDanglingTargets(t *testing.T) {
	fixture := setupEscalatorDB(t)
	definition := fmt.Sprintf(`{"schemaVersion":1,"steps":[{"targets":[{"type":"schedule","id":9999},{"type":"user","id":%d}],"delayMinutes":5}],"repeatCount":0}`, fixture.Alice)
	policyId := createPolicy(t, fixture.OrgId, definition)
	page := openTestPageForPolicy(t, fixture, policyId, "rule6|/issues/abc")

	tickAndDrain(t, time.Now().UTC())

	rows := pageNotifications(t, page.Id)
	if len(rows) != 1 {
		t.Fatalf("expected the dangling schedule to be skipped and alice notified, got %d rows", len(rows))
	}
	if rows[0].UserId == nil || *rows[0].UserId != fixture.Alice {
		t.Errorf("notified user = %v, want alice", rows[0].UserId)
	}
}

func TestEscalatorUsesConfiguredContactMethods(t *testing.T) {
	fixture := setupEscalatorDB(t)
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.UserContactMethodRepository.Create(tx, &models.UserContactMethod{
			UserId:     fixture.Alice,
			MethodType: "email",
			Config:     models.JSONText(`{"email":"pager@example.com"}`),
			Enabled:    true,
			Verified:   true,
			CreatedAt:  time.Now().UTC(),
		})
	})
	if err != nil {
		t.Fatalf("create contact method: %v", err)
	}

	policyId := createPolicy(t, fixture.OrgId, twoStepPolicy(fixture.Alice, fixture.Bob))
	page := openTestPageForPolicy(t, fixture, policyId, "rule7|/issues/abc")
	tickAndDrain(t, time.Now().UTC())

	rows := pageNotifications(t, page.Id)
	if len(rows) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(rows))
	}
	if rows[0].TargetDesc != "pager@example.com (email)" {
		t.Errorf("target desc = %q, want the override email", rows[0].TargetDesc)
	}
}

func TestPolicyValidation(t *testing.T) {
	fixture := setupEscalatorDB(t)
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		cases := []struct {
			name string
			json string
		}{
			{"no steps", `{"steps":[],"repeatCount":0}`},
			{"no targets", `{"steps":[{"targets":[],"delayMinutes":5}]}`},
			{"delay too small", fmt.Sprintf(`{"steps":[{"targets":[{"type":"user","id":%d}],"delayMinutes":0}]}`, fixture.Alice)},
			{"bad repeat", fmt.Sprintf(`{"steps":[{"targets":[{"type":"user","id":%d}],"delayMinutes":5}],"repeatCount":99}`, fixture.Alice)},
			{"unknown target type", `{"steps":[{"targets":[{"type":"pigeon","id":1}],"delayMinutes":5}]}`},
			{"nonexistent user", `{"steps":[{"targets":[{"type":"user","id":424242}],"delayMinutes":5}]}`},
			{"nonexistent schedule", `{"steps":[{"targets":[{"type":"schedule","id":424242}],"delayMinutes":5}]}`},
		}
		for _, tc := range cases {
			if _, err := ValidatePolicyDefinition(tx, fixture.OrgId, []byte(tc.json)); err == nil {
				t.Errorf("expected validation error for %s", tc.name)
			}
		}
		valid := fmt.Sprintf(`{"steps":[{"targets":[{"type":"user","id":%d}],"delayMinutes":15}],"repeatCount":1}`, fixture.Alice)
		if _, err := ValidatePolicyDefinition(tx, fixture.OrgId, []byte(valid)); err != nil {
			t.Errorf("expected valid policy to pass, got %v", err)
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
}

func TestOpenPageRequiresMatchingOrg(t *testing.T) {
	fixture := setupEscalatorDB(t)
	otherOrgId := 0
	otherProject := uuid.Nil
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		org, err := transactional.OrganizationRepository.Create(tx, "Other", "UTC")
		if err != nil {
			return struct{}{}, err
		}
		otherOrgId = org.Id
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "other-api", "gin", org.Id)
		if err != nil {
			return struct{}{}, err
		}
		otherProject = project.Id
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}
	_ = otherOrgId

	policyId := createPolicy(t, fixture.OrgId, twoStepPolicy(fixture.Alice, fixture.Bob))
	_, err = openPage(openPageParams{
		PolicyId:  policyId,
		ProjectId: otherProject,
		RuleName:  "r",
		RuleType:  "new_error",
		Subject:   "s",
		DedupKey:  "cross-org",
	})
	if err == nil {
		t.Error("expected cross-org page open to fail")
	}
}
