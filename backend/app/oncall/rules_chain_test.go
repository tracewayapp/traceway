//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb

package oncall

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/outbox"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
)

func TestResolveUrgencyMatrix(t *testing.T) {
	cases := []struct {
		policy   string
		severity string
		want     string
	}{
		{"", "critical", models.UrgencyHigh},
		{"", "", models.UrgencyHigh},
		{"", "warning", models.UrgencyLow},
		{"", "info", models.UrgencyLow},
		{"auto", "critical", models.UrgencyHigh},
		{"auto", "info", models.UrgencyLow},
		{"high", "info", models.UrgencyHigh},
		{"low", "critical", models.UrgencyLow},
		{"junk", "critical", models.UrgencyHigh},
	}
	for _, tc := range cases {
		if got := ResolveUrgency(tc.policy, tc.severity); got != tc.want {
			t.Errorf("ResolveUrgency(%q, %q) = %q, want %q", tc.policy, tc.severity, got, tc.want)
		}
	}
}

func TestPolicyUrgencyValidationAndParsing(t *testing.T) {
	fixture := setupEscalatorDB(t)
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		valid := `{"steps":[{"targets":[{"type":"user","id":` + itoa(fixture.Alice) + `}],"delayMinutes":5}],"urgency":"high"}`
		if _, err := ValidatePolicyDefinition(tx, fixture.OrgId, []byte(valid)); err != nil {
			t.Errorf("high urgency should validate: %v", err)
		}
		bad := `{"steps":[{"targets":[{"type":"user","id":` + itoa(fixture.Alice) + `}],"delayMinutes":5}],"urgency":"shout"}`
		if _, err := ValidatePolicyDefinition(tx, fixture.OrgId, []byte(bad)); err == nil {
			t.Error("junk urgency should 422")
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}

	// Old definitions without the field parse to auto.
	definition, err := ParsePolicyDefinition([]byte(`{"schemaVersion":1,"steps":[],"repeatCount":0}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if definition.Urgency != "" {
		t.Errorf("legacy definition urgency = %q, want empty (auto)", definition.Urgency)
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func createContactMethod(t *testing.T, userId int, methodType string, config string, enabled bool, verified bool) int {
	t.Helper()
	id, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.UserContactMethodRepository.Create(tx, &models.UserContactMethod{
			UserId:     userId,
			MethodType: methodType,
			Config:     models.JSONText(config),
			Enabled:    enabled,
			Verified:   verified,
			CreatedAt:  time.Now().UTC(),
		})
	})
	if err != nil {
		t.Fatalf("create contact method: %v", err)
	}
	return id
}

func setRules(t *testing.T, userId int, urgency string, steps []models.UserNotificationRule) {
	t.Helper()
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		rules := make([]*models.UserNotificationRule, 0, len(steps))
		for i := range steps {
			rule := steps[i]
			rule.UserId = userId
			rule.Urgency = urgency
			rule.Position = i
			rule.CreatedAt = time.Now().UTC()
			rules = append(rules, &rule)
		}
		return struct{}{}, transactional.UserNotificationRuleRepository.ReplaceForUser(tx, userId, rules)
	})
	if err != nil {
		t.Fatalf("set rules: %v", err)
	}
}

func outboxRowsForPage(t *testing.T, pageId int) []*models.OutboxDelivery {
	t.Helper()
	rows, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.OutboxDelivery, error) {
		return transactional.OutboxRepository.FindCancellable(tx, outbox.PageCancelKey(pageId))
	})
	if err != nil {
		t.Fatalf("load outbox rows: %v", err)
	}
	return rows
}

func singleStepPolicy(userId int) string {
	return `{"schemaVersion":1,"steps":[{"targets":[{"type":"user","id":` + strconv.Itoa(userId) + `}],"delayMinutes":30}],"repeatCount":0}`
}

// enableTwilioForTest makes SMS a sendable method for the duration of one
// test; without credentials the escalator skips SMS methods entirely.
func enableTwilioForTest(t *testing.T) {
	t.Helper()
	previous := *config.Config
	config.Config.TwilioAccountSID = "ACtest"
	config.Config.TwilioAuthToken = "test-token"
	config.Config.TwilioFromNumber = "+15005550006"
	t.Cleanup(func() { *config.Config = previous })
}

func TestRuleChainEnqueuesStaggeredSteps(t *testing.T) {
	fixture := setupEscalatorDB(t)
	enableTwilioForTest(t)
	emailId := createContactMethod(t, fixture.Alice, "email", `{}`, true, true)
	smsId := createContactMethod(t, fixture.Alice, "sms", `{"phoneNumber":"+12025550123"}`, true, true)
	setRules(t, fixture.Alice, models.UrgencyHigh, []models.UserNotificationRule{
		{ContactMethodId: smsId, DelayMinutes: 0},
		{ContactMethodId: emailId, DelayMinutes: 5},
	})

	policyId := createPolicy(t, fixture.OrgId, singleStepPolicy(fixture.Alice))
	page := openTestPageForPolicy(t, fixture, policyId, "chain1|/issues/abc")

	now := time.Now().UTC()
	runEscalatorTick(context.Background(), now)

	rows := pageNotifications(t, page.Id)
	if len(rows) != 2 {
		t.Fatalf("expected 2 chain rows, got %d", len(rows))
	}
	if rows[0].MethodType != "sms" || rows[0].ScheduledFor == nil {
		t.Errorf("first step = %+v, want immediate sms", rows[0])
	}
	if rows[1].MethodType != "email" || rows[1].ScheduledFor == nil {
		t.Fatalf("second step = %+v, want delayed email", rows[1])
	}
	if delay := rows[1].ScheduledFor.Sub(now); delay < 4*time.Minute || delay > 6*time.Minute {
		t.Errorf("email step scheduled in %v, want ~5m", delay)
	}
	if rows[0].AckTokenHash == "" || rows[1].AckTokenHash == "" || rows[0].AckTokenHash == rows[1].AckTokenHash {
		t.Error("each delivery row should carry a distinct ack token hash")
	}

	outboxRows := outboxRowsForPage(t, page.Id)
	if len(outboxRows) != 2 {
		t.Fatalf("expected 2 outbox rows, got %d", len(outboxRows))
	}
	var delayed *models.OutboxDelivery
	for _, row := range outboxRows {
		if row.AdapterType == "email" {
			delayed = row
		}
	}
	if delayed == nil {
		t.Fatal("expected an email outbox row")
	}
	if gap := delayed.NextAttemptAt.Sub(now); gap < 4*time.Minute || gap > 6*time.Minute {
		t.Errorf("email outbox NotBefore in %v, want ~5m", gap)
	}
}

func TestAckCancelsChainTail(t *testing.T) {
	fixture := setupEscalatorDB(t)
	emailId := createContactMethod(t, fixture.Alice, "email", `{}`, true, true)
	setRules(t, fixture.Alice, models.UrgencyHigh, []models.UserNotificationRule{
		{ContactMethodId: emailId, DelayMinutes: 0},
		{ContactMethodId: emailId, DelayMinutes: 10},
	})

	policyId := createPolicy(t, fixture.OrgId, singleStepPolicy(fixture.Alice))
	page := openTestPageForPolicy(t, fixture, policyId, "chain2|/issues/abc")

	now := time.Now().UTC()
	tickAndDrain(t, now)

	rows := pageNotifications(t, page.Id)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Status != models.PageNotificationSent {
		t.Errorf("immediate step = %s, want sent", rows[0].Status)
	}
	if rows[1].Status != models.PageNotificationPending {
		t.Errorf("delayed step = %s, want pending", rows[1].Status)
	}

	acknowledged, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return AcknowledgePage(tx, page.Id, &fixture.Alice, AckViaDashboard, now)
	})
	if err != nil || !acknowledged {
		t.Fatalf("acknowledge: %v (%v)", err, acknowledged)
	}
	rows = pageNotifications(t, page.Id)
	if rows[1].Status != models.PageNotificationCancelled {
		t.Errorf("delayed step after ack = %s, want cancelled", rows[1].Status)
	}
	page = reloadPage(t, page.Id)
	if page.AcknowledgedVia != AckViaDashboard {
		t.Errorf("acknowledged_via = %q, want dashboard", page.AcknowledgedVia)
	}
}

func TestRuleChainSkipsUnusableStepsAndFallsBack(t *testing.T) {
	fixture := setupEscalatorDB(t)
	disabledId := createContactMethod(t, fixture.Alice, "email", `{}`, false, true)
	unverifiedSmsId := createContactMethod(t, fixture.Alice, "sms", `{"phoneNumber":"+12025550123"}`, true, false)
	setRules(t, fixture.Alice, models.UrgencyHigh, []models.UserNotificationRule{
		{ContactMethodId: disabledId, DelayMinutes: 0},
		{ContactMethodId: unverifiedSmsId, DelayMinutes: 5},
	})

	policyId := createPolicy(t, fixture.OrgId, singleStepPolicy(fixture.Alice))
	page := openTestPageForPolicy(t, fixture, policyId, "chain3|/issues/abc")
	runEscalatorTick(context.Background(), time.Now().UTC())

	rows := pageNotifications(t, page.Id)
	if len(rows) != 1 {
		t.Fatalf("expected 1 fallback row, got %d", len(rows))
	}
	if rows[0].MethodType != "email" || rows[0].TargetDesc != "alice@example.com (email)" {
		t.Errorf("fallback should page the account email, got %+v", rows[0])
	}
}

// An SMS method can outlive its transport: the credentials are removed after
// the method was verified. The page must not vanish into an unsendable
// channel, so SMS is skipped and the account-email fallback still fires.
func TestSmsMethodsSkippedWithoutTwilio(t *testing.T) {
	fixture := setupEscalatorDB(t)
	if config.Config.TwilioEnabled() {
		t.Skip("Twilio configured in this environment")
	}
	smsId := createContactMethod(t, fixture.Alice, "sms", `{"phoneNumber":"+12025550123"}`, true, true)
	setRules(t, fixture.Alice, models.UrgencyHigh, []models.UserNotificationRule{{ContactMethodId: smsId, DelayMinutes: 0}})

	policyId := createPolicy(t, fixture.OrgId, singleStepPolicy(fixture.Alice))
	page := openTestPageForPolicy(t, fixture, policyId, "nosms|/issues/abc")
	runEscalatorTick(context.Background(), time.Now().UTC())

	rows := pageNotifications(t, page.Id)
	if len(rows) != 1 {
		t.Fatalf("expected 1 fallback row, got %d", len(rows))
	}
	if rows[0].MethodType != "email" || rows[0].TargetDesc != "alice@example.com (email)" {
		t.Errorf("unsendable sms should fall back to the account email, got %+v", rows[0])
	}
}

func TestLowUrgencyPageRunsLowChain(t *testing.T) {
	fixture := setupEscalatorDB(t)
	emailId := createContactMethod(t, fixture.Alice, "email", `{"email":"low@example.com"}`, true, true)
	smsId := createContactMethod(t, fixture.Alice, "sms", `{"phoneNumber":"+12025550123"}`, true, true)
	setRules(t, fixture.Alice, models.UrgencyHigh, []models.UserNotificationRule{{ContactMethodId: smsId, DelayMinutes: 0}})
	// setRules replaces the WHOLE rule set, so write both chains at once.
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		now := time.Now().UTC()
		return struct{}{}, transactional.UserNotificationRuleRepository.ReplaceForUser(tx, fixture.Alice, []*models.UserNotificationRule{
			{UserId: fixture.Alice, Urgency: models.UrgencyHigh, Position: 0, DelayMinutes: 0, ContactMethodId: smsId, CreatedAt: now},
			{UserId: fixture.Alice, Urgency: models.UrgencyLow, Position: 0, DelayMinutes: 0, ContactMethodId: emailId, CreatedAt: now},
		})
	})
	if err != nil {
		t.Fatalf("set chains: %v", err)
	}

	// warning severity + auto urgency -> low chain (email), not sms.
	policyId := createPolicy(t, fixture.OrgId, singleStepPolicy(fixture.Alice))
	if _, err := openPage(openPageParams{
		PolicyId: policyId, ProjectId: fixture.ProjectId, RuleName: "r", RuleType: "new_error",
		Subject: "warn", Severity: "warning", DedupKey: "chain4|/issues/w",
	}); err != nil {
		t.Fatalf("open page: %v", err)
	}
	page := findPageByDedupKey(t, "chain4|/issues/w")
	if page.Urgency != models.UrgencyLow {
		t.Fatalf("page urgency = %q, want low", page.Urgency)
	}
	runEscalatorTick(context.Background(), time.Now().UTC())

	rows := pageNotifications(t, page.Id)
	if len(rows) != 1 || rows[0].MethodType != "email" || rows[0].TargetDesc != "low@example.com (email)" {
		t.Errorf("low chain should run the email step, got %+v", rows)
	}
}

func TestAckTokenFlow(t *testing.T) {
	fixture := setupEscalatorDB(t)
	emailId := createContactMethod(t, fixture.Alice, "email", `{}`, true, true)
	setRules(t, fixture.Alice, models.UrgencyHigh, []models.UserNotificationRule{{ContactMethodId: emailId, DelayMinutes: 0}})

	policyId := createPolicy(t, fixture.OrgId, singleStepPolicy(fixture.Alice))
	page := openTestPageForPolicy(t, fixture, policyId, "token1|/issues/abc")
	runEscalatorTick(context.Background(), time.Now().UTC())

	// The plaintext token exists only inside the outgoing message.
	outboxRows := outboxRowsForPage(t, page.Id)
	if len(outboxRows) != 1 {
		t.Fatalf("expected 1 outbox row, got %d", len(outboxRows))
	}
	var msg models.NotificationMessage
	if err := jsonUnmarshalForTest(outboxRows[0].Message, &msg); err != nil {
		t.Fatalf("decode message: %v", err)
	}
	idx := strings.LastIndex(msg.URL, "/ack/")
	if idx < 0 {
		t.Fatalf("message URL carries no ack link: %q", msg.URL)
	}
	token := msg.URL[idx+len("/ack/"):]
	if !strings.HasPrefix(token, "twk_") {
		t.Fatalf("unexpected token shape %q", token)
	}

	// Token resolves to the delivery row, attributed to Alice.
	notification, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.PageNotification, error) {
		return transactional.PageNotificationRepository.FindByAckTokenHash(tx, shared.HashAuthToken(token))
	})
	if err != nil || notification == nil {
		t.Fatalf("token lookup failed: %v (%v)", err, notification)
	}
	if notification.UserId == nil || *notification.UserId != fixture.Alice {
		t.Fatalf("token attributed to %v, want alice", notification.UserId)
	}

	// Link-ack records via + attribution and stops the escalation.
	acknowledged, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return AcknowledgePage(tx, page.Id, notification.UserId, AckViaLink, time.Now().UTC())
	})
	if err != nil || !acknowledged {
		t.Fatalf("link ack failed: %v (%v)", err, acknowledged)
	}
	page = reloadPage(t, page.Id)
	if page.AcknowledgedVia != AckViaLink || page.AcknowledgedBy == nil || *page.AcknowledgedBy != fixture.Alice {
		t.Errorf("link ack not recorded: via=%q by=%v", page.AcknowledgedVia, page.AcknowledgedBy)
	}

	// Wrong token finds nothing.
	missing, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.PageNotification, error) {
		return transactional.PageNotificationRepository.FindByAckTokenHash(tx, shared.HashAuthToken("twk_bogus"))
	})
	if err != nil || missing != nil {
		t.Errorf("bogus token should resolve to nothing, got %v (%v)", missing, err)
	}
}

func jsonUnmarshalForTest(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
