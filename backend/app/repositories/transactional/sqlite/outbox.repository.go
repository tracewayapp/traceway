//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type outboxRepository struct{}

const outboxColumns = "id, kind, status, adapter_type, adapter_config, message, attempts, next_attempt_at, claimed_at, cancel_key, page_notification_id, rule_id, project_id, channel_name, last_error, created_at, sent_at"

// Enqueue inserts a pending row in the caller's transaction and returns its id.
// The commit of that transaction is the durable "someone will be notified"
// promise.
func (r *outboxRepository) Enqueue(tx *sql.Tx, row *models.OutboxDelivery) (int, error) {
	return lit.Insert[models.OutboxDelivery](tx, row)
}

func (r *outboxRepository) FindById(tx *sql.Tx, id int) (*models.OutboxDelivery, error) {
	return lit.SelectSingleNamed[models.OutboxDelivery](
		tx,
		"SELECT "+outboxColumns+" FROM notification_outbox WHERE id = :id",
		lit.P{"id": id},
	)
}

// FindDue returns pending rows whose next_attempt_at has passed: pages first,
// then oldest first.
func (r *outboxRepository) FindDue(tx *sql.Tx, now time.Time, limit int) ([]*models.OutboxDelivery, error) {
	return lit.SelectNamed[models.OutboxDelivery](
		tx,
		"SELECT "+outboxColumns+" FROM notification_outbox WHERE status = 'pending' AND next_attempt_at <= :now ORDER BY CASE WHEN kind = 'page' THEN 0 ELSE 1 END, next_attempt_at ASC, id ASC LIMIT :limit",
		lit.P{"now": now.UTC(), "limit": limit},
	)
}

// MarkSending claims one row: pending -> sending, attempts+1. The status guard
// makes the claim lose against a concurrent cancel; returns whether the claim
// won, and callers must not send when it lost.
func (r *outboxRepository) MarkSending(tx *sql.Tx, id int, now time.Time) (bool, error) {
	return guardedStatusUpdate(
		tx,
		"UPDATE notification_outbox SET status = 'sending', attempts = attempts + 1, claimed_at = :now WHERE id = :id AND status = 'pending'",
		lit.P{"now": now.UTC(), "id": id},
	)
}

// MarkSent finalizes a delivered row. The status guard loses to a concurrent
// cancel: a cancelled row stays cancelled even when the last send landed.
// Returns whether the row was still sending.
func (r *outboxRepository) MarkSent(tx *sql.Tx, id int, now time.Time) (bool, error) {
	return guardedStatusUpdate(
		tx,
		"UPDATE notification_outbox SET status = 'sent', sent_at = :now, last_error = '' WHERE id = :id AND status = 'sending'",
		lit.P{"now": now.UTC(), "id": id},
	)
}

// MarkFailedWithBackoff records a failed attempt. nextAttemptAt == nil is
// terminal (status failed); otherwise the row returns to pending, scheduled at
// nextAttemptAt. Guarded on status = 'sending' so cancel wins races; returns
// whether the row was still sending.
func (r *outboxRepository) MarkFailedWithBackoff(tx *sql.Tx, id int, errorMsg string, nextAttemptAt *time.Time, now time.Time) (bool, error) {
	if nextAttemptAt == nil {
		return guardedStatusUpdate(
			tx,
			"UPDATE notification_outbox SET status = 'failed', last_error = :error_msg, sent_at = :now WHERE id = :id AND status = 'sending'",
			lit.P{"error_msg": errorMsg, "now": now.UTC(), "id": id},
		)
	}
	return guardedStatusUpdate(
		tx,
		"UPDATE notification_outbox SET status = 'pending', last_error = :error_msg, next_attempt_at = :next_attempt_at, claimed_at = NULL WHERE id = :id AND status = 'sending'",
		lit.P{"error_msg": errorMsg, "next_attempt_at": nextAttemptAt.UTC(), "id": id},
	)
}

// guardedStatusUpdate runs a status-guarded UPDATE and reports whether it
// matched: zero rows means a concurrent transition (usually a cancel or a
// lost ack/resolve race) won.
func guardedStatusUpdate(tx *sql.Tx, namedQuery string, params lit.P) (bool, error) {
	query, args, err := lit.ParseNamedQuery(db.Driver, namedQuery, params)
	if err != nil {
		return false, err
	}
	result, err := tx.Exec(query, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// FindCancellable returns the pending/sending rows for a cancel key, so their
// linked page_notifications can be mirrored before the status flip.
func (r *outboxRepository) FindCancellable(tx *sql.Tx, cancelKey string) ([]*models.OutboxDelivery, error) {
	return lit.SelectNamed[models.OutboxDelivery](
		tx,
		"SELECT "+outboxColumns+" FROM notification_outbox WHERE cancel_key <> '' AND cancel_key = :cancel_key AND status IN ('pending', 'sending') ORDER BY id ASC",
		lit.P{"cancel_key": cancelKey},
	)
}

// CancelByKey flips every pending/sending row holding the key to cancelled.
func (r *outboxRepository) CancelByKey(tx *sql.Tx, cancelKey string, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE notification_outbox SET status = 'cancelled', sent_at = :now WHERE cancel_key <> '' AND cancel_key = :cancel_key AND status IN ('pending', 'sending')",
		lit.P{"now": now.UTC(), "cancel_key": cancelKey},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

// CancelByProject cancels a deleted project's queued rule deliveries; page
// rows carry no project id and are cancelled by key.
func (r *outboxRepository) CancelByProject(tx *sql.Tx, projectId uuid.UUID, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE notification_outbox SET status = 'cancelled', sent_at = :now WHERE project_id = :project_id AND status IN ('pending', 'sending')",
		lit.P{"now": now.UTC(), "project_id": projectId},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

// ReclaimStaleSending returns rows claimed before the cutoff (a run died
// between claim-commit and result-commit) to pending, due immediately.
// Attempts are NOT reset, so crash loops still reach terminal failure.
func (r *outboxRepository) ReclaimStaleSending(tx *sql.Tx, cutoff time.Time, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE notification_outbox SET status = 'pending', next_attempt_at = :now, claimed_at = NULL WHERE status = 'sending' AND claimed_at < :cutoff",
		lit.P{"now": now.UTC(), "cutoff": cutoff.UTC()},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

// LastEnqueuedPerRule backstops cooldown seeding at boot: the newest outbox
// row per rule regardless of status (fired_notifications only exists once an
// outcome is terminal). The max is folded in Go because SQLite loses column
// type affinity on aggregates; the table is small (terminal rows are pruned).
func (r *outboxRepository) LastEnqueuedPerRule(tx *sql.Tx) (map[int]time.Time, error) {
	rows, err := lit.SelectNamed[models.OutboxRuleEnqueue](
		tx,
		"SELECT rule_id, created_at AS last_enqueued_at FROM notification_outbox WHERE rule_id IS NOT NULL",
		lit.P{},
	)
	if err != nil {
		return nil, err
	}
	result := make(map[int]time.Time, len(rows))
	for _, row := range rows {
		if existing, ok := result[row.RuleId]; !ok || row.LastEnqueuedAt.After(existing) {
			result[row.RuleId] = row.LastEnqueuedAt
		}
	}
	return result, nil
}

func (r *outboxRepository) CountsForHealth(tx *sql.Tx) (*models.OutboxHealthCounts, error) {
	return lit.SelectSingleNamed[models.OutboxHealthCounts](
		tx,
		"SELECT COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) AS pending_count, COALESCE(SUM(CASE WHEN status = 'sending' THEN 1 ELSE 0 END), 0) AS sending_count, COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) AS failed_count FROM notification_outbox",
		lit.P{},
	)
}

// OldestPending returns the pending row with the earliest next_attempt_at, or
// nil. A plain-column select, because SQLite loses type affinity on
// timestamp aggregates.
func (r *outboxRepository) OldestPending(tx *sql.Tx) (*models.OutboxDelivery, error) {
	return lit.SelectSingleNamed[models.OutboxDelivery](
		tx,
		"SELECT "+outboxColumns+" FROM notification_outbox WHERE status = 'pending' ORDER BY next_attempt_at ASC, id ASC LIMIT 1",
		lit.P{},
	)
}

// PruneTerminal deletes finished rows past retention.
func (r *outboxRepository) PruneTerminal(tx *sql.Tx, sentCutoff time.Time, failedCutoff time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM notification_outbox WHERE (status IN ('sent', 'cancelled') AND created_at < :sent_cutoff) OR (status = 'failed' AND created_at < :failed_cutoff)",
		lit.P{"sent_cutoff": sentCutoff.UTC(), "failed_cutoff": failedCutoff.UTC()},
	)
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

var OutboxRepository = outboxRepository{}
