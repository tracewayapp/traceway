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

type syntheticCheckRepository struct{}

const syntheticCheckColumns = "id, project_id, name, check_type, enabled, interval_seconds, timeout_seconds, config, failure_threshold, current_status, consecutive_failures, last_run_at, last_state_change_at, next_run_at, created_at, updated_at"

func (r *syntheticCheckRepository) Create(tx *sql.Tx, check *models.SyntheticCheck) (int, error) {
	return lit.Insert[models.SyntheticCheck](tx, check)
}

// Update writes only the user-editable columns. The run-state columns
// (current_status, consecutive_failures, last_run_at, last_state_change_at)
// belong to ApplyRunState; a full-row update here could revert a concurrent
// executor finalization on PostgreSQL.
func (r *syntheticCheckRepository) Update(tx *sql.Tx, check *models.SyntheticCheck) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE synthetic_checks SET name = :name, enabled = :enabled, interval_seconds = :interval_seconds, timeout_seconds = :timeout_seconds, config = :config, failure_threshold = :failure_threshold, next_run_at = :next_run_at, updated_at = :updated_at WHERE id = :id",
		lit.P{
			"name":              check.Name,
			"enabled":           check.Enabled,
			"interval_seconds":  check.IntervalSeconds,
			"timeout_seconds":   check.TimeoutSeconds,
			"config":            check.Config,
			"failure_threshold": check.FailureThreshold,
			"next_run_at":       check.NextRunAt.UTC(),
			"updated_at":        check.UpdatedAt.UTC(),
			"id":                check.Id,
		},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *syntheticCheckRepository) Delete(tx *sql.Tx, id int, projectId uuid.UUID) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM synthetic_checks WHERE id = :id AND project_id = :project_id", lit.P{"id": id, "project_id": projectId})
}

func (r *syntheticCheckRepository) FindById(tx *sql.Tx, id int) (*models.SyntheticCheck, error) {
	return lit.SelectSingleNamed[models.SyntheticCheck](
		tx,
		"SELECT "+syntheticCheckColumns+" FROM synthetic_checks WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *syntheticCheckRepository) FindByIdForProject(tx *sql.Tx, id int, projectId uuid.UUID) (*models.SyntheticCheck, error) {
	return lit.SelectSingleNamed[models.SyntheticCheck](
		tx,
		"SELECT "+syntheticCheckColumns+" FROM synthetic_checks WHERE id = :id AND project_id = :project_id",
		lit.P{"id": id, "project_id": projectId},
	)
}

func (r *syntheticCheckRepository) FindByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.SyntheticCheck, error) {
	return lit.SelectNamed[models.SyntheticCheck](
		tx,
		"SELECT "+syntheticCheckColumns+" FROM synthetic_checks WHERE project_id = :project_id ORDER BY name ASC, id ASC",
		lit.P{"project_id": projectId},
	)
}

// FindDueEnabled returns enabled checks whose next run time has passed,
// oldest-due first.
func (r *syntheticCheckRepository) FindDueEnabled(tx *sql.Tx, now time.Time, limit int) ([]*models.SyntheticCheck, error) {
	return lit.SelectNamed[models.SyntheticCheck](
		tx,
		"SELECT "+syntheticCheckColumns+" FROM synthetic_checks WHERE enabled = true AND next_run_at <= :now ORDER BY next_run_at ASC, id ASC LIMIT :limit",
		lit.P{"now": now.UTC(), "limit": limit},
	)
}

// SetNextRun advances the schedule pointer. Called in the same transaction
// that enqueues the run row, so multi-instance schedulers (behind the
// advisory lock) never double-enqueue an interval.
func (r *syntheticCheckRepository) SetNextRun(tx *sql.Tx, id int, nextRunAt time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE synthetic_checks SET next_run_at = :next_run_at WHERE id = :id",
		lit.P{"next_run_at": nextRunAt.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

// ApplyRunState persists the outcome of one executed run on the check row.
// lastStateChangeAt is only written on transitions (nil leaves it untouched).
func (r *syntheticCheckRepository) ApplyRunState(tx *sql.Tx, id int, currentStatus string, consecutiveFailures int, lastRunAt time.Time, lastStateChangeAt *time.Time) error {
	if lastStateChangeAt != nil {
		query, args, err := lit.ParseNamedQuery(
			db.Driver,
			"UPDATE synthetic_checks SET current_status = :current_status, consecutive_failures = :consecutive_failures, last_run_at = :last_run_at, last_state_change_at = :last_state_change_at WHERE id = :id",
			lit.P{"current_status": currentStatus, "consecutive_failures": consecutiveFailures, "last_run_at": lastRunAt.UTC(), "last_state_change_at": lastStateChangeAt.UTC(), "id": id},
		)
		if err != nil {
			return err
		}
		return lit.UpdateNative(tx, query, args...)
	}
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE synthetic_checks SET current_status = :current_status, consecutive_failures = :consecutive_failures, last_run_at = :last_run_at WHERE id = :id",
		lit.P{"current_status": currentStatus, "consecutive_failures": consecutiveFailures, "last_run_at": lastRunAt.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

// FindByIdInOrganization scopes a check lookup to an organization (status
// pages reference checks across all the org's projects).
func (r *syntheticCheckRepository) FindByIdInOrganization(tx *sql.Tx, id int, organizationId int) (*models.SyntheticCheck, error) {
	return lit.SelectSingleNamed[models.SyntheticCheck](
		tx,
		"SELECT c.id, c.project_id, c.name, c.check_type, c.enabled, c.interval_seconds, c.timeout_seconds, c.config, c.failure_threshold, c.current_status, c.consecutive_failures, c.last_run_at, c.last_state_change_at, c.next_run_at, c.created_at, c.updated_at FROM synthetic_checks c JOIN projects p ON p.id = c.project_id WHERE c.id = :id AND p.organization_id = :organization_id",
		lit.P{"id": id, "organization_id": organizationId},
	)
}

func (r *syntheticCheckRepository) FindByOrganization(tx *sql.Tx, organizationId int) ([]*models.SyntheticCheck, error) {
	return lit.SelectNamed[models.SyntheticCheck](
		tx,
		"SELECT c.id, c.project_id, c.name, c.check_type, c.enabled, c.interval_seconds, c.timeout_seconds, c.config, c.failure_threshold, c.current_status, c.consecutive_failures, c.last_run_at, c.last_state_change_at, c.next_run_at, c.created_at, c.updated_at FROM synthetic_checks c JOIN projects p ON p.id = c.project_id WHERE p.organization_id = :organization_id ORDER BY c.name ASC, c.id ASC",
		lit.P{"organization_id": organizationId},
	)
}

func (r *syntheticCheckRepository) CountDownByOrganization(tx *sql.Tx, organizationId int) (int, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM synthetic_checks c JOIN projects p ON p.id = c.project_id WHERE p.organization_id = :organization_id AND c.enabled = true AND c.current_status = 'down'",
		lit.P{"organization_id": organizationId},
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.Count, nil
}

func (r *syntheticCheckRepository) CountDownAll(tx *sql.Tx) (int, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM synthetic_checks WHERE enabled = true AND current_status = 'down'",
		lit.P{},
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.Count, nil
}

func (r *syntheticCheckRepository) CountDownByProject(tx *sql.Tx, projectId uuid.UUID) (int, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM synthetic_checks WHERE project_id = :project_id AND enabled = true AND current_status = 'down'",
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

var SyntheticCheckRepository = syntheticCheckRepository{}
