//go:build !transactional_pg && !telemetry_ch && !telemetry_duckdb && !telemetry_firebolt

package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/dbtest"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

func setupOutboxDB(t *testing.T) {
	t.Helper()
	dbtest.SetupSQLite(t)
	t.Cleanup(func() {
		sender = nil
		terminalHook = nil
	})
}

type fakeSender struct {
	mu    sync.Mutex
	calls []fakeCall
	fail  error
}

type fakeCall struct {
	AdapterType string
	Config      string
	Message     models.NotificationMessage
}

func (f *fakeSender) fn(ctx context.Context, adapterType string, adapterConfig json.RawMessage, msg models.NotificationMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{AdapterType: adapterType, Config: string(adapterConfig), Message: msg})
	return f.fail
}

func (f *fakeSender) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func enqueueTest(t *testing.T, d Delivery) int {
	t.Helper()
	id, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return Enqueue(tx, d)
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	return id
}

func reloadRow(t *testing.T, id int) *models.OutboxDelivery {
	t.Helper()
	row, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.OutboxDelivery, error) {
		return transactional.OutboxRepository.FindById(tx, id)
	})
	if err != nil || row == nil {
		t.Fatalf("reload outbox row %d: %v", id, err)
	}
	return row
}

func testMessage(subject string) models.NotificationMessage {
	return models.NotificationMessage{Subject: subject, Body: "body", Severity: models.NotificationSeverityCritical}
}

func TestEnqueueSetsPendingAndDue(t *testing.T) {
	setupOutboxDB(t)
	now := time.Now().UTC()

	immediate := enqueueTest(t, Delivery{Kind: models.OutboxKindRule, AdapterType: "slack", AdapterConfig: json.RawMessage(`{"webhookUrl":"x"}`), Message: testMessage("a")})
	future := now.Add(30 * time.Minute)
	scheduled := enqueueTest(t, Delivery{Kind: models.OutboxKindPage, AdapterType: "email", Message: testMessage("b"), NotBefore: &future})

	due, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.OutboxDelivery, error) {
		return transactional.OutboxRepository.FindDue(tx, now.Add(time.Second), 10)
	})
	if err != nil {
		t.Fatalf("find due: %v", err)
	}
	if len(due) != 1 || due[0].Id != immediate {
		t.Fatalf("expected only the immediate row due, got %+v", due)
	}

	dueAtFuture, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.OutboxDelivery, error) {
		return transactional.OutboxRepository.FindDue(tx, future.Add(time.Second), 10)
	})
	if err != nil {
		t.Fatalf("find due future: %v", err)
	}
	if len(dueAtFuture) != 2 {
		t.Fatalf("expected both rows due at NotBefore, got %d", len(dueAtFuture))
	}
	if row := reloadRow(t, scheduled); row.Status != models.OutboxPending {
		t.Errorf("scheduled row status = %s, want pending", row.Status)
	}
}

func TestDrainTickSendsViaRegisteredSender(t *testing.T) {
	setupOutboxDB(t)
	fake := &fakeSender{}
	RegisterSender(fake.fn)

	id := enqueueTest(t, Delivery{Kind: models.OutboxKindRule, AdapterType: "slack", AdapterConfig: json.RawMessage(`{"webhookUrl":"https://example.com"}`), Message: testMessage("hello")})
	DrainOnce(context.Background(), time.Now().UTC())

	if fake.callCount() != 1 {
		t.Fatalf("sender calls = %d, want 1", fake.callCount())
	}
	call := fake.calls[0]
	if call.AdapterType != "slack" || call.Config != `{"webhookUrl":"https://example.com"}` || call.Message.Subject != "hello" {
		t.Errorf("sender got %+v", call)
	}
	row := reloadRow(t, id)
	if row.Status != models.OutboxSent || row.SentAt == nil {
		t.Errorf("row after send = %s (sent_at %v), want sent", row.Status, row.SentAt)
	}
}

func TestBackoffProgressionToTerminal(t *testing.T) {
	setupOutboxDB(t)
	fake := &fakeSender{fail: errors.New("boom")}
	RegisterSender(fake.fn)
	var hookStatus string
	var hookCalls int
	RegisterTerminalHook(func(row *models.OutboxDelivery, status string, errorMsg string) {
		hookCalls++
		hookStatus = status
	})

	id := enqueueTest(t, Delivery{Kind: models.OutboxKindRule, AdapterType: "slack", Message: testMessage("x")})

	now := time.Now().UTC()
	expectedDelays := []time.Duration{1 * time.Minute, 5 * time.Minute, 15 * time.Minute, 60 * time.Minute}
	for attempt := 1; attempt <= 4; attempt++ {
		DrainOnce(context.Background(), now)
		row := reloadRow(t, id)
		if row.Status != models.OutboxPending {
			t.Fatalf("after attempt %d status = %s, want pending", attempt, row.Status)
		}
		if row.Attempts != attempt {
			t.Fatalf("after attempt %d attempts = %d", attempt, row.Attempts)
		}
		delay := row.NextAttemptAt.Sub(time.Now().UTC())
		want := expectedDelays[attempt-1]
		if delay < want-time.Minute || delay > want+time.Minute {
			t.Errorf("attempt %d backoff = %v, want ~%v", attempt, delay, want)
		}
		now = row.NextAttemptAt.Add(time.Second)
	}

	DrainOnce(context.Background(), now)
	row := reloadRow(t, id)
	if row.Status != models.OutboxFailed {
		t.Fatalf("after attempt 5 status = %s, want failed", row.Status)
	}
	if row.LastError == "" {
		t.Error("terminal row should keep last_error")
	}
	if hookCalls != 1 || hookStatus != models.OutboxFailed {
		t.Errorf("terminal hook calls = %d status = %s, want 1 failed", hookCalls, hookStatus)
	}
}

func TestCrashAfterClaimIsReclaimed(t *testing.T) {
	setupOutboxDB(t)
	fake := &fakeSender{}
	RegisterSender(fake.fn)

	id := enqueueTest(t, Delivery{Kind: models.OutboxKindRule, AdapterType: "slack", Message: testMessage("x")})

	// Simulate a crash between claim-commit and send: claim without finalize.
	now := time.Now().UTC()
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		_, err := transactional.OutboxRepository.MarkSending(tx, id, now)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatalf("mark sending: %v", err)
	}

	// Within the stale window nothing happens.
	DrainOnce(context.Background(), now.Add(1*time.Minute))
	if fake.callCount() != 0 {
		t.Fatal("row should not be re-sent inside the stale-sending window")
	}

	// Past the window the row is reclaimed and delivered; attempts kept.
	DrainOnce(context.Background(), now.Add(staleSendingAge+time.Second))
	if fake.callCount() != 1 {
		t.Fatalf("sender calls = %d, want 1 after reclaim", fake.callCount())
	}
	row := reloadRow(t, id)
	if row.Status != models.OutboxSent {
		t.Errorf("status = %s, want sent", row.Status)
	}
	if row.Attempts != 2 {
		t.Errorf("attempts = %d, want 2 (claim before crash + reclaimed attempt)", row.Attempts)
	}
}

// seedPage creates the minimal org -> project -> page chain so FK-constrained
// page_notifications rows can exist.
func seedPage(t *testing.T) int {
	t.Helper()
	pageId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		org, err := transactional.OrganizationRepository.Create(tx, "Acme", "UTC")
		if err != nil {
			return 0, err
		}
		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, "api", "gin", org.Id)
		if err != nil {
			return 0, err
		}
		now := time.Now().UTC()
		return transactional.PageRepository.Create(tx, &models.Page{
			OrganizationId: org.Id, ProjectId: project.Id,
			PolicySnapshot: models.JSONText("{}"), Subject: "s", Status: models.PageStatusOpen,
			DedupKey: "test", EventCount: 1, LastEventAt: now, EscalationLevel: -1,
			CreatedAt: now, UpdatedAt: now,
		})
	})
	if err != nil {
		t.Fatalf("seed page: %v", err)
	}
	return pageId
}

func TestCancelByKeyCancelsPendingAndMirrors(t *testing.T) {
	setupOutboxDB(t)
	pageId := seedPage(t)

	notificationId, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
		return transactional.PageNotificationRepository.Create(tx, &models.PageNotification{
			PageId: pageId, Level: 0, TargetDesc: "x", MethodType: "email",
			Status: models.PageNotificationPending, CreatedAt: time.Now().UTC(),
		})
	})
	if err != nil {
		t.Fatalf("create page notification: %v", err)
	}
	id := enqueueTest(t, Delivery{Kind: models.OutboxKindPage, AdapterType: "email", Message: testMessage("x"), CancelKey: PageCancelKey(pageId), PageNotificationId: &notificationId})
	sentId := enqueueTest(t, Delivery{Kind: models.OutboxKindPage, AdapterType: "email", Message: testMessage("y"), CancelKey: PageCancelKey(pageId)})
	now := time.Now().UTC()
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		if _, err := transactional.OutboxRepository.MarkSending(tx, sentId, now); err != nil {
			return struct{}{}, err
		}
		_, err := transactional.OutboxRepository.MarkSent(tx, sentId, now)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatalf("pre-send row: %v", err)
	}

	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, CancelByKey(tx, PageCancelKey(1))
	})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if row := reloadRow(t, id); row.Status != models.OutboxCancelled {
		t.Errorf("pending row status = %s, want cancelled", row.Status)
	}
	if row := reloadRow(t, sentId); row.Status != models.OutboxSent {
		t.Errorf("already-sent row status = %s, want sent (untouched)", row.Status)
	}
	notifications, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.PageNotification, error) {
		return transactional.PageNotificationRepository.FindByPage(tx, pageId)
	})
	if err != nil || len(notifications) != 1 {
		t.Fatalf("load notifications: %v (%d)", err, len(notifications))
	}
	if notifications[0].Status != models.PageNotificationCancelled {
		t.Errorf("mirrored notification status = %s, want cancelled", notifications[0].Status)
	}
}

func TestCancelBeatsInFlightSend(t *testing.T) {
	setupOutboxDB(t)
	id := enqueueTest(t, Delivery{Kind: models.OutboxKindPage, AdapterType: "email", Message: testMessage("x"), CancelKey: PageCancelKey(2)})
	now := time.Now().UTC()

	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		_, err := transactional.OutboxRepository.MarkSending(tx, id, now)
		return struct{}{}, err
	})
	if err != nil {
		t.Fatalf("mark sending: %v", err)
	}
	_, err = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, CancelByKey(tx, PageCancelKey(2))
	})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	// The in-flight send finishes and tries to mark sent: the guard loses.
	sent, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		return transactional.OutboxRepository.MarkSent(tx, id, now)
	})
	if err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	if sent {
		t.Error("MarkSent reported a win against a cancelled row")
	}
	if row := reloadRow(t, id); row.Status != models.OutboxCancelled {
		t.Errorf("status = %s, want cancelled to win the race", row.Status)
	}
}

func TestLastEnqueuedPerRuleAndHealthAndPrune(t *testing.T) {
	setupOutboxDB(t)
	ruleId := 7
	enqueueTest(t, Delivery{Kind: models.OutboxKindRule, AdapterType: "slack", Message: testMessage("x"), RuleId: &ruleId})

	last, err := db.ExecuteTransaction(func(tx *sql.Tx) (map[int]time.Time, error) {
		return transactional.OutboxRepository.LastEnqueuedPerRule(tx)
	})
	if err != nil {
		t.Fatalf("last enqueued: %v", err)
	}
	if _, ok := last[ruleId]; !ok {
		t.Errorf("expected rule %d in seeding map, got %v", ruleId, last)
	}

	stats, err := HealthSnapshot()
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if stats.Pending != 1 {
		t.Errorf("health pending = %d, want 1", stats.Pending)
	}

	// Prune: sent row 8 days old pruned; failed row 8 days old kept; failed 31d pruned; pending never.
	old := time.Now().UTC().AddDate(0, 0, -8)
	veryOld := time.Now().UTC().AddDate(0, 0, -31)
	mkRow := func(status string, createdAt time.Time) int {
		id, err := db.ExecuteTransaction(func(tx *sql.Tx) (int, error) {
			return transactional.OutboxRepository.Enqueue(tx, &models.OutboxDelivery{
				Kind: models.OutboxKindRule, Status: status, AdapterType: "slack",
				AdapterConfig: models.JSONText("{}"), Message: models.JSONText("{}"),
				NextAttemptAt: createdAt, CreatedAt: createdAt,
			})
		})
		if err != nil {
			t.Fatalf("insert %s row: %v", status, err)
		}
		return id
	}
	sentOld := mkRow(models.OutboxSent, old)
	failedOld := mkRow(models.OutboxFailed, old)
	failedVeryOld := mkRow(models.OutboxFailed, veryOld)

	pruned, err := db.ExecuteTransaction(func(tx *sql.Tx) (int64, error) {
		now := time.Now().UTC()
		return transactional.OutboxRepository.PruneTerminal(tx, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30))
	})
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 2 {
		t.Errorf("pruned = %d, want 2 (old sent + very old failed)", pruned)
	}
	if row, _ := db.ExecuteTransaction(func(tx *sql.Tx) (*models.OutboxDelivery, error) {
		return transactional.OutboxRepository.FindById(tx, failedOld)
	}); row == nil {
		t.Error("8-day-old failed row should be kept (30d retention)")
	}
	for _, gone := range []int{sentOld, failedVeryOld} {
		if row, _ := db.ExecuteTransaction(func(tx *sql.Tx) (*models.OutboxDelivery, error) {
			return transactional.OutboxRepository.FindById(tx, gone)
		}); row != nil {
			t.Errorf("row %d should be pruned", gone)
		}
	}
}
