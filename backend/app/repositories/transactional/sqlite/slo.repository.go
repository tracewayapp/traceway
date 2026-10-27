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

type sloRepository struct{}

const sloColumns = "id, project_id, name, description, sli_kind, sli_config, target_percent, window_days, bucket_minutes, enabled, status, current_sli, budget_remaining, burn_rate_1h, burn_rate_6h, computed_at, history_from, last_bucket_at, sli_since, gap_from, gap_to, revision, created_by, created_at, updated_at"

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

func nullableFloat(f *float64) any {
	if f == nil {
		return nil
	}
	return *f
}

func (r *sloRepository) Create(tx *sql.Tx, slo *models.SLO) (int, error) {
	return lit.Insert[models.SLO](tx, slo)
}

func (r *sloRepository) Update(tx *sql.Tx, slo *models.SLO) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE slos SET name = :name, description = :description, sli_kind = :sli_kind, sli_config = :sli_config, target_percent = :target_percent, window_days = :window_days, bucket_minutes = :bucket_minutes, enabled = :enabled, updated_at = :updated_at, revision = revision + 1 WHERE id = :id",
		lit.P{
			"name":           slo.Name,
			"description":    slo.Description,
			"sli_kind":       slo.SliKind,
			"sli_config":     slo.SliConfig,
			"target_percent": slo.TargetPercent,
			"window_days":    slo.WindowDays,
			"bucket_minutes": slo.BucketMinutes,
			"enabled":        slo.Enabled,
			"updated_at":     slo.UpdatedAt.UTC(),
			"id":             slo.Id,
		},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *sloRepository) ResetHistory(tx *sql.Tx, id int, now time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE slos SET status = 'no_data', current_sli = NULL, budget_remaining = NULL, burn_rate_1h = NULL, burn_rate_6h = NULL, computed_at = NULL, history_from = NULL, last_bucket_at = NULL, gap_from = NULL, gap_to = NULL, sli_since = :sli_since WHERE id = :id",
		lit.P{"id": id, "sli_since": now.UTC()},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *sloRepository) UpdateSnapshot(tx *sql.Tx, id int, revision int, snapshot models.SLOSnapshot) (bool, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE slos SET status = :status, current_sli = :current_sli, budget_remaining = :budget_remaining, burn_rate_1h = :burn_rate_1h, burn_rate_6h = :burn_rate_6h, computed_at = :computed_at, history_from = :history_from, last_bucket_at = :last_bucket_at, gap_from = :gap_from, gap_to = :gap_to WHERE id = :id AND revision = :revision",
		lit.P{
			"status":           snapshot.Status,
			"current_sli":      nullableFloat(snapshot.CurrentSli),
			"budget_remaining": nullableFloat(snapshot.BudgetRemaining),
			"burn_rate_1h":     nullableFloat(snapshot.BurnRate1h),
			"burn_rate_6h":     nullableFloat(snapshot.BurnRate6h),
			"computed_at":      snapshot.ComputedAt.UTC(),
			"history_from":     nullableTime(snapshot.HistoryFrom),
			"last_bucket_at":   nullableTime(snapshot.LastBucketAt),
			"gap_from":         nullableTime(snapshot.GapFrom),
			"gap_to":           nullableTime(snapshot.GapTo),
			"id":               id,
			"revision":         revision,
		},
	)
	if err != nil {
		return false, err
	}
	result, err := tx.Exec(query, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (r *sloRepository) Delete(tx *sql.Tx, id int, projectId uuid.UUID) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM slos WHERE id = :id AND project_id = :project_id", lit.P{"id": id, "project_id": projectId})
}

func (r *sloRepository) FindById(tx *sql.Tx, id int) (*models.SLO, error) {
	return lit.SelectSingleNamed[models.SLO](
		tx,
		"SELECT "+sloColumns+" FROM slos WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *sloRepository) FindByIdForProject(tx *sql.Tx, id int, projectId uuid.UUID) (*models.SLO, error) {
	return lit.SelectSingleNamed[models.SLO](
		tx,
		"SELECT "+sloColumns+" FROM slos WHERE id = :id AND project_id = :project_id",
		lit.P{"id": id, "project_id": projectId},
	)
}

func (r *sloRepository) FindByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.SLO, error) {
	return lit.SelectNamed[models.SLO](
		tx,
		"SELECT "+sloColumns+" FROM slos WHERE project_id = :project_id ORDER BY name ASC, id ASC",
		lit.P{"project_id": projectId},
	)
}

func (r *sloRepository) FindEnabled(tx *sql.Tx) ([]*models.SLO, error) {
	return lit.SelectNamed[models.SLO](
		tx,
		"SELECT "+sloColumns+" FROM slos WHERE enabled = true ORDER BY id ASC",
		lit.P{},
	)
}

func (r *sloRepository) FindEnabledFreshness(tx *sql.Tx) ([]*models.SLOFreshness, error) {
	return lit.SelectNamed[models.SLOFreshness](
		tx,
		"SELECT id, window_days, computed_at, history_from, last_bucket_at, gap_from, created_at FROM slos WHERE enabled = true",
		lit.P{},
	)
}

func (r *sloRepository) CountBreachedByProject(tx *sql.Tx, projectId uuid.UUID) (int, error) {
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM slos WHERE project_id = :project_id AND enabled = true AND status = 'breached'",
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

var SLORepository = &sloRepository{}
