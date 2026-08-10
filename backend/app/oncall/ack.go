package oncall

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/outbox"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

const (
	AckViaDashboard = "dashboard"
	AckViaLink      = "link"
)

// AcknowledgePage transitions open -> acknowledged and cancels every queued
// or retrying delivery under the page's cancel key (mirroring their delivery
// log rows). Returns false when the page was not open. The single home for
// ack-side cancellation: used by the dashboard controller and the tokenized
// ack endpoint alike. userId is nil for anonymous link-acks.
func AcknowledgePage(tx *sql.Tx, pageId int, userId *int, via string, now time.Time) (bool, error) {
	acknowledged, err := transactional.PageRepository.Acknowledge(tx, pageId, userId, via, now)
	if err != nil || !acknowledged {
		return acknowledged, err
	}
	return true, outbox.CancelByKey(tx, outbox.PageCancelKey(pageId))
}

// ResolvePage transitions open/acknowledged -> resolved and cancels queued
// deliveries. Returns false when the page was already resolved.
func ResolvePage(tx *sql.Tx, pageId int, userId int, now time.Time) (bool, error) {
	resolved, err := transactional.PageRepository.Resolve(tx, pageId, userId, now)
	if err != nil || !resolved {
		return resolved, err
	}
	return true, outbox.CancelByKey(tx, outbox.PageCancelKey(pageId))
}
