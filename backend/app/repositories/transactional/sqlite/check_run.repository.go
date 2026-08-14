//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type checkRunRepository struct{}

const checkRunColumns = "id, check_id, project_id, check_type, status, scheduled_for, expires_at, claimed_at, claimed_by, created_at"

// Enqueue inserts a queued run in the caller's transaction and returns its id.
func (r *checkRunRepository) Enqueue(tx *sql.Tx, run *models.CheckRun) (int, error) {
	return lit.Insert[models.CheckRun](tx, run)
}

func (r *checkRunRepository) FindById(tx *sql.Tx, id int) (*models.CheckRun, error) {
	return lit.SelectSingleNamed[models.CheckRun](
		tx,
		"SELECT "+checkRunColumns+" FROM check_runs WHERE id = :id",
		lit.P{"id": id},
	)
}

// FindClaimable returns queued, unexpired runs of the given types, oldest
// first. Types are matched positionally because lit has no IN-list expansion
// for named params; callers pass 1-3 types, padding is harmless.
func (r *checkRunRepository) FindClaimable(tx *sql.Tx, typeA string, typeB string, typeC string, now time.Time, limit int) ([]*models.CheckRun, error) {
	return lit.SelectNamed[models.CheckRun](
		tx,
		"SELECT "+checkRunColumns+" FROM check_runs WHERE status = 'queued' AND expires_at > :now AND check_type IN (:type_a, :type_b, :type_c) ORDER BY scheduled_for ASC, id ASC LIMIT :limit",
		lit.P{"now": now.UTC(), "type_a": typeA, "type_b": typeB, "type_c": typeC, "limit": limit},
	)
}

// FindClaimableBrowser returns queued, unexpired browser runs for the
// remote-runner poll endpoint. Runners are operator infrastructure serving
// the whole instance, so there is deliberately no tenant filter.
func (r *checkRunRepository) FindClaimableBrowser(tx *sql.Tx, now time.Time, limit int) ([]*models.CheckRun, error) {
	return lit.SelectNamed[models.CheckRun](
		tx,
		"SELECT id, check_id, project_id, check_type, status, scheduled_for, expires_at, claimed_at, claimed_by, created_at FROM check_runs WHERE status = 'queued' AND check_type = 'browser' AND expires_at > :now ORDER BY scheduled_for ASC, id ASC LIMIT :limit",
		lit.P{"now": now.UTC(), "limit": limit},
	)
}

// Claim transitions one run queued -> claimed. The status guard makes the
// claim lose against a concurrent claimer; returns whether the claim won,
// and callers must not execute when it lost.
func (r *checkRunRepository) Claim(tx *sql.Tx, id int, claimedBy string, now time.Time) (bool, error) {
	return guardedStatusUpdate(
		tx,
		"UPDATE check_runs SET status = 'claimed', claimed_at = :now, claimed_by = :claimed_by WHERE id = :id AND status = 'queued'",
		lit.P{"now": now.UTC(), "claimed_by": claimedBy, "id": id},
	)
}

// Delete removes a finalized run. The telemetry check_results row is the
// durable record; terminal queue rows are never kept.
func (r *checkRunRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM check_runs WHERE id = :id", lit.P{"id": id})
}

// DeleteClaimedBy finalizes a run only while the caller still owns the claim.
// Zero rows means a stale-claim reclaim handed the run to someone else; the
// loser must discard its outcome instead of double-applying state.
func (r *checkRunRepository) DeleteClaimedBy(tx *sql.Tx, id int, claimedBy string) (bool, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM check_runs WHERE id = :id AND status = 'claimed' AND claimed_by = :claimed_by",
		lit.P{"id": id, "claimed_by": claimedBy},
	)
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

// DeleteQueuedByCheck drops not-yet-claimed runs, used when a check is
// paused so stale probes don't fire on re-enable.
func (r *checkRunRepository) DeleteQueuedByCheck(tx *sql.Tx, checkId int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM check_runs WHERE check_id = :check_id AND status = 'queued'", lit.P{"check_id": checkId})
}

// ReclaimStaleClaimed returns runs claimed before the cutoff (the claimer
// died mid-execution) to queued so another executor can pick them up; their
// expires_at still bounds how stale a probe may run.
func (r *checkRunRepository) ReclaimStaleClaimed(tx *sql.Tx, cutoff time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE check_runs SET status = 'queued', claimed_at = NULL, claimed_by = '' WHERE status = 'claimed' AND claimed_at < :cutoff",
		lit.P{"cutoff": cutoff.UTC()},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

// FindExpiredQueued returns queued runs past their expiry, to be recorded as
// missed and deleted. Executing them late would corrupt the uptime timeline.
func (r *checkRunRepository) FindExpiredQueued(tx *sql.Tx, now time.Time, limit int) ([]*models.CheckRun, error) {
	return lit.SelectNamed[models.CheckRun](
		tx,
		"SELECT "+checkRunColumns+" FROM check_runs WHERE status = 'queued' AND expires_at <= :now ORDER BY expires_at ASC, id ASC LIMIT :limit",
		lit.P{"now": now.UTC(), "limit": limit},
	)
}

// HasPendingForCheck reports whether a queued or claimed run already exists,
// so run-now cannot pile up duplicates behind a stuck executor.
func (r *checkRunRepository) HasPendingForCheck(tx *sql.Tx, checkId int) (bool, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM check_runs WHERE check_id = :check_id AND status IN ('queued', 'claimed')",
		lit.P{"check_id": checkId},
	)
	if err != nil {
		return false, err
	}
	return result != nil && result.Count > 0, nil
}

func (r *checkRunRepository) CountsForHealth(tx *sql.Tx) (*models.CheckRunHealthCounts, error) {
	return lit.SelectSingleNamed[models.CheckRunHealthCounts](
		tx,
		"SELECT COALESCE(SUM(CASE WHEN status = 'queued' THEN 1 ELSE 0 END), 0) AS queued_count, COALESCE(SUM(CASE WHEN status = 'claimed' THEN 1 ELSE 0 END), 0) AS claimed_count FROM check_runs",
		lit.P{},
	)
}

// OldestQueued returns the queued run with the earliest scheduled_for, or
// nil. A plain-column select, because SQLite loses type affinity on
// timestamp aggregates.
func (r *checkRunRepository) OldestQueued(tx *sql.Tx) (*models.CheckRun, error) {
	return lit.SelectSingleNamed[models.CheckRun](
		tx,
		"SELECT "+checkRunColumns+" FROM check_runs WHERE status = 'queued' ORDER BY scheduled_for ASC, id ASC LIMIT 1",
		lit.P{},
	)
}

var CheckRunRepository = checkRunRepository{}
