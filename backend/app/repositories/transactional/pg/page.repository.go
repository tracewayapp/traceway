//go:build transactional_pg

package pg

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type pageRepository struct{}

const pageColumns = "id, organization_id, project_id, policy_id, policy_snapshot, rule_id, rule_name, rule_type, subject, body, url, severity, urgency, status, dedup_key, event_count, last_event_at, escalation_level, repeat_iteration, next_escalation_at, last_escalated_at, acknowledged_by, acknowledged_via, acknowledged_at, resolved_by, resolved_at, created_at, updated_at"

func (r *pageRepository) FindById(tx *sql.Tx, id int) (*models.Page, error) {
	return lit.SelectSingleNamed[models.Page](
		tx,
		"SELECT "+pageColumns+" FROM pages WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *pageRepository) FindUnresolvedByDedupKey(tx *sql.Tx, dedupKey string) (*models.Page, error) {
	return lit.SelectSingleNamed[models.Page](
		tx,
		"SELECT "+pageColumns+" FROM pages WHERE dedup_key = :dedup_key AND status <> 'resolved'",
		lit.P{"dedup_key": dedupKey},
	)
}

func (r *pageRepository) BumpEvent(tx *sql.Tx, id int, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE pages SET event_count = event_count + 1, last_event_at = :now, updated_at = :now WHERE id = :id",
		lit.P{"now": now.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

// FindDueById re-fetches one page only if it is still due, so a per-page claim
// transaction can skip pages acknowledged, resolved, or claimed by a
// concurrent escalator since the due list was read.
func (r *pageRepository) FindDueById(tx *sql.Tx, id int, now time.Time) (*models.Page, error) {
	return lit.SelectSingleNamed[models.Page](
		tx,
		"SELECT "+pageColumns+" FROM pages WHERE id = :id AND status = 'open' AND next_escalation_at IS NOT NULL AND next_escalation_at <= :now",
		lit.P{"id": id, "now": now.UTC()},
	)
}

// FindDue returns open pages whose next escalation is due, oldest first.
func (r *pageRepository) FindDue(tx *sql.Tx, now time.Time) ([]*models.Page, error) {
	return lit.SelectNamed[models.Page](
		tx,
		"SELECT "+pageColumns+" FROM pages WHERE status = 'open' AND next_escalation_at IS NOT NULL AND next_escalation_at <= :now ORDER BY next_escalation_at ASC, id ASC",
		lit.P{"now": now.UTC()},
	)
}

// FindUnresolvedByIssueHash returns the unresolved pages opened for one
// exception hash: new-error/regression pages carry the hash as the dedup token
// after the "ruleId|" prefix, so the suffix match is exact for those rule
// types. Served by the pages_project_active_idx partial index.
func (r *pageRepository) FindUnresolvedByIssueHash(tx *sql.Tx, projectId uuid.UUID, hash string) ([]*models.Page, error) {
	return lit.SelectNamed[models.Page](
		tx,
		"SELECT "+pageColumns+" FROM pages WHERE project_id = :project_id AND status <> 'resolved' AND (rule_type = 'new_error' OR rule_type = 'error_regression') AND dedup_key LIKE ('%|' || :hash)",
		lit.P{"project_id": projectId, "hash": hash},
	)
}

func (r *pageRepository) FindByProject(tx *sql.Tx, projectId uuid.UUID, status string, limit int, offset int) ([]*models.Page, error) {
	return lit.SelectNamed[models.Page](
		tx,
		"SELECT "+pageColumns+" FROM pages WHERE project_id = :project_id AND ("+statusCondition(status)+") ORDER BY created_at DESC, id DESC LIMIT :limit OFFSET :offset",
		lit.P{"project_id": projectId, "limit": limit, "offset": offset},
	)
}

func (r *pageRepository) CountByProject(tx *sql.Tx, projectId uuid.UUID, status string) (int, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM pages WHERE project_id = :project_id AND ("+statusCondition(status)+")",
		lit.P{"project_id": projectId},
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.Count, nil
}

func (r *pageRepository) CountOpenByProject(tx *sql.Tx, projectId uuid.UUID) (int, error) {
	return r.CountByProject(tx, projectId, models.PageStatusOpen)
}

// statusCondition maps a status filter to a fixed SQL condition; values are
// from a closed set, never user input.
func statusCondition(status string) string {
	switch status {
	case models.PageStatusOpen:
		return "status = 'open'"
	case models.PageStatusAcknowledged:
		return "status = 'acknowledged'"
	case models.PageStatusResolved:
		return "status = 'resolved'"
	case "active":
		return "status = 'open' OR status = 'acknowledged'"
	default:
		return "1 = 1"
	}
}

func (r *pageRepository) Create(tx *sql.Tx, page *models.Page) (int, error) {
	return lit.Insert[models.Page](tx, page)
}

// UpdateEscalationState advances the escalation clock. Guarded on
// status = 'open' so a claim racing a concurrent acknowledge/resolve loses:
// returns false when the page is no longer open, and the caller must roll the
// claim back (its inserted deliveries were invisible to the ack's cancel).
func (r *pageRepository) UpdateEscalationState(tx *sql.Tx, id int, level int, iteration int, nextEscalationAt *time.Time, now time.Time) (bool, error) {
	var nextValue any
	if nextEscalationAt != nil {
		nextValue = nextEscalationAt.UTC()
	}
	return guardedStatusUpdate(
		tx,
		"UPDATE pages SET escalation_level = :level, repeat_iteration = :iteration, next_escalation_at = :next_at, last_escalated_at = :now, updated_at = :now WHERE id = :id AND status = 'open'",
		lit.P{"level": level, "iteration": iteration, "next_at": nextValue, "now": now.UTC(), "id": id},
	)
}

// Acknowledge transitions open -> acknowledged. Returns false when the page
// was not open (lost race or wrong state). userId is nil for anonymous
// link-acks; via is 'dashboard' or 'link'.
func (r *pageRepository) Acknowledge(tx *sql.Tx, id int, userId *int, via string, now time.Time) (bool, error) {
	var userValue any
	if userId != nil {
		userValue = *userId
	}
	return guardedStatusUpdate(
		tx,
		"UPDATE pages SET status = 'acknowledged', acknowledged_by = :user_id, acknowledged_via = :via, acknowledged_at = :now, next_escalation_at = NULL, updated_at = :now WHERE id = :id AND status = 'open'",
		lit.P{"user_id": userValue, "via": via, "now": now.UTC(), "id": id},
	)
}

// Resolve transitions open/acknowledged -> resolved. Returns false when the
// page was already resolved.
func (r *pageRepository) Resolve(tx *sql.Tx, id int, userId int, now time.Time) (bool, error) {
	return guardedStatusUpdate(
		tx,
		"UPDATE pages SET status = 'resolved', resolved_by = :user_id, resolved_at = :now, next_escalation_at = NULL, updated_at = :now WHERE id = :id AND status <> 'resolved'",
		lit.P{"user_id": userId, "now": now.UTC(), "id": id},
	)
}

// ResolveSystem resolves a page with no resolving user, for system-driven
// resolution (a synthetic check recovering). Same guard as Resolve.
func (r *pageRepository) ResolveSystem(tx *sql.Tx, id int, now time.Time) (bool, error) {
	return guardedStatusUpdate(
		tx,
		"UPDATE pages SET status = 'resolved', resolved_by = NULL, resolved_at = :now, next_escalation_at = NULL, updated_at = :now WHERE id = :id AND status <> 'resolved'",
		lit.P{"now": now.UTC(), "id": id},
	)
}

var PageRepository = pageRepository{}
