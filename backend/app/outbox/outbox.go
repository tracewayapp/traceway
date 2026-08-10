package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// Delivery is one intended notification send. Enqueue persists it; the drain
// worker delivers it with retries. AdapterConfig is a snapshot: it is never
// re-derived at send time, so later channel/contact-method edits do not affect
// rows already queued.
type Delivery struct {
	Kind               string
	AdapterType        string
	AdapterConfig      json.RawMessage
	Message            models.NotificationMessage
	NotBefore          *time.Time
	CancelKey          string
	PageNotificationId *int
	RuleId             *int
	ProjectId          *uuid.UUID
	ChannelName        string
}

// Enqueue inserts a pending outbox row in the caller's transaction. The commit
// of that transaction is the durable "someone will be notified" promise.
// Callers should Wake() after their transaction commits.
func Enqueue(tx *sql.Tx, d Delivery) (int, error) {
	messageJSON, err := json.Marshal(d.Message)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	next := now
	if d.NotBefore != nil {
		next = d.NotBefore.UTC()
	}
	cfg := d.AdapterConfig
	if len(cfg) == 0 {
		cfg = json.RawMessage("{}")
	}
	row := &models.OutboxDelivery{
		Kind:               d.Kind,
		Status:             models.OutboxPending,
		AdapterType:        d.AdapterType,
		AdapterConfig:      models.JSONText(cfg),
		Message:            models.JSONText(messageJSON),
		NextAttemptAt:      next,
		CancelKey:          d.CancelKey,
		PageNotificationId: d.PageNotificationId,
		RuleId:             d.RuleId,
		ProjectId:          d.ProjectId,
		ChannelName:        d.ChannelName,
		CreatedAt:          now,
	}
	return transactional.OutboxRepository.Enqueue(tx, row)
}

// CancelByKey flips every pending/sending row under the key to cancelled and
// mirrors their linked page_notifications rows. Runs in the caller's
// transaction. An in-flight send may still deliver once (at-least-once), but
// the row's final state stays cancelled: MarkSent/MarkFailedWithBackoff are
// guarded on status = 'sending'.
func CancelByKey(tx *sql.Tx, cancelKey string) error {
	now := time.Now().UTC()
	rows, err := transactional.OutboxRepository.FindCancellable(tx, cancelKey)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.PageNotificationId != nil {
			if err := transactional.PageNotificationRepository.MarkCancelled(tx, *row.PageNotificationId, now); err != nil {
				return err
			}
		}
	}
	return transactional.OutboxRepository.CancelByKey(tx, cancelKey, now)
}

func PageCancelKey(pageId int) string {
	return "page:" + strconv.Itoa(pageId)
}

var wakeCh = make(chan struct{}, 1)

// Wake nudges the drain worker so a freshly enqueued delivery goes out
// immediately instead of waiting for the next poll. Non-blocking.
func Wake() {
	select {
	case wakeCh <- struct{}{}:
	default:
	}
}

// SendFunc performs one delivery attempt. Registered from cmd/run.go with the
// notifications-package implementation; the indirection exists because this
// package cannot import notifications (notifications imports it to enqueue).
type SendFunc func(ctx context.Context, adapterType string, adapterConfig json.RawMessage, msg models.NotificationMessage) error

var sender SendFunc

func RegisterSender(fn SendFunc) {
	sender = fn
}

// TerminalHook observes terminal outcomes (models.OutboxSent or
// models.OutboxFailed) for audit bookkeeping. Called outside any transaction;
// must not block.
type TerminalHook func(row *models.OutboxDelivery, status string, errorMsg string)

var terminalHook TerminalHook

func RegisterTerminalHook(fn TerminalHook) {
	terminalHook = fn
}
