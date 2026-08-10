//go:build transactional_pg

package pg

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type pageNotificationRepository struct{}

const pageNotificationColumns = "id, page_id, level, iteration, user_id, target_desc, method_type, status, error_msg, scheduled_for, ack_token_hash, created_at, sent_at"

func (r *pageNotificationRepository) FindByPage(tx *sql.Tx, pageId int) ([]*models.PageNotification, error) {
	return lit.SelectNamed[models.PageNotification](
		tx,
		"SELECT "+pageNotificationColumns+" FROM page_notifications WHERE page_id = :page_id ORDER BY created_at ASC, id ASC",
		lit.P{"page_id": pageId},
	)
}

// FindByAckTokenHash resolves a delivery ack token. The non-empty guard means
// a row without a token (channel deliveries) can never match, even if a caller
// ever hashes an empty input.
func (r *pageNotificationRepository) FindByAckTokenHash(tx *sql.Tx, hash string) (*models.PageNotification, error) {
	return lit.SelectSingleNamed[models.PageNotification](
		tx,
		"SELECT "+pageNotificationColumns+" FROM page_notifications WHERE ack_token_hash = :hash AND ack_token_hash <> ''",
		lit.P{"hash": hash},
	)
}

func (r *pageNotificationRepository) Create(tx *sql.Tx, notification *models.PageNotification) (int, error) {
	return lit.Insert[models.PageNotification](tx, notification)
}

// MarkSent finalizes a delivered row. Guarded on status = 'pending' so a
// terminal state (cancelled/failed) is never rewritten; the drain's mirror
// call tolerates matching nothing.
func (r *pageNotificationRepository) MarkSent(tx *sql.Tx, id int, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE page_notifications SET status = 'sent', sent_at = :now WHERE id = :id AND status = 'pending'",
		lit.P{"now": now.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

// MarkCancelled flips a not-yet-delivered row to cancelled; already-sent or
// failed rows are left untouched.
func (r *pageNotificationRepository) MarkCancelled(tx *sql.Tx, id int, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE page_notifications SET status = 'cancelled', sent_at = :now WHERE id = :id AND status = 'pending'",
		lit.P{"now": now.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

// MarkFailed records a terminal delivery failure. Guarded on
// status = 'pending' so a cancelled row is never resurrected to failed.
func (r *pageNotificationRepository) MarkFailed(tx *sql.Tx, id int, errorMsg string, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE page_notifications SET status = 'failed', error_msg = :error_msg, sent_at = :now WHERE id = :id AND status = 'pending'",
		lit.P{"error_msg": errorMsg, "now": now.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

var PageNotificationRepository = pageNotificationRepository{}
