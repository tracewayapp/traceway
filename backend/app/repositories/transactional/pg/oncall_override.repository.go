//go:build transactional_pg

package pg

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type oncallOverrideRepository struct{}

const oncallOverrideColumns = "id, schedule_id, user_id, start_at, end_at, created_by, created_at"

func (r *oncallOverrideRepository) FindById(tx *sql.Tx, id int) (*models.OncallOverride, error) {
	return lit.SelectSingleNamed[models.OncallOverride](
		tx,
		"SELECT "+oncallOverrideColumns+" FROM oncall_overrides WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *oncallOverrideRepository) ListForRange(tx *sql.Tx, scheduleId int, from time.Time, to time.Time) ([]*models.OncallOverride, error) {
	return lit.SelectNamed[models.OncallOverride](
		tx,
		`SELECT `+oncallOverrideColumns+` FROM oncall_overrides
		WHERE schedule_id = :schedule_id AND end_at > :from AND start_at < :to
		ORDER BY created_at ASC, id ASC`,
		lit.P{"schedule_id": scheduleId, "from": from.UTC(), "to": to.UTC()},
	)
}

// ListForRangeByOrganization returns every override intersecting [from, to)
// across the organization's schedules in one query; callers group by
// ScheduleId.
func (r *oncallOverrideRepository) ListForRangeByOrganization(tx *sql.Tx, organizationId int, from time.Time, to time.Time) ([]*models.OncallOverride, error) {
	return lit.SelectNamed[models.OncallOverride](
		tx,
		`SELECT `+oncallOverrideColumns+` FROM oncall_overrides
		WHERE schedule_id IN (SELECT id FROM oncall_schedules WHERE organization_id = :organization_id) AND end_at > :from AND start_at < :to
		ORDER BY created_at ASC, id ASC`,
		lit.P{"organization_id": organizationId, "from": from.UTC(), "to": to.UTC()},
	)
}

func (r *oncallOverrideRepository) Create(tx *sql.Tx, override *models.OncallOverride) (int, error) {
	return lit.Insert[models.OncallOverride](tx, override)
}

func (r *oncallOverrideRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM oncall_overrides WHERE id = :id", lit.P{"id": id})
}

// DeleteByOrganizationAndUser removes a user's overrides across every schedule
// in the organization; called when the user is removed from the organization.
func (r *oncallOverrideRepository) DeleteByOrganizationAndUser(tx *sql.Tx, organizationId int, userId int) error {
	return lit.DeleteNamed(
		db.Driver,
		tx,
		"DELETE FROM oncall_overrides WHERE user_id = :user_id AND schedule_id IN (SELECT id FROM oncall_schedules WHERE organization_id = :organization_id)",
		lit.P{"user_id": userId, "organization_id": organizationId},
	)
}

var OncallOverrideRepository = oncallOverrideRepository{}
