//go:build transactional_pg

package pg

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type oncallScheduleRepository struct{}

const oncallScheduleColumns = "id, organization_id, team_id, name, description, timezone, definition, created_by, created_at, updated_at"

func (r *oncallScheduleRepository) FindById(tx *sql.Tx, id int) (*models.OncallSchedule, error) {
	return lit.SelectSingleNamed[models.OncallSchedule](
		tx,
		"SELECT "+oncallScheduleColumns+" FROM oncall_schedules WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *oncallScheduleRepository) FindByOrganizationAndName(tx *sql.Tx, organizationId int, name string) (*models.OncallSchedule, error) {
	return lit.SelectSingleNamed[models.OncallSchedule](
		tx,
		"SELECT "+oncallScheduleColumns+" FROM oncall_schedules WHERE organization_id = :organization_id AND LOWER(name) = LOWER(:name)",
		lit.P{"organization_id": organizationId, "name": name},
	)
}

func (r *oncallScheduleRepository) ListByOrganization(tx *sql.Tx, organizationId int) ([]*models.OncallSchedule, error) {
	return lit.SelectNamed[models.OncallSchedule](
		tx,
		"SELECT "+oncallScheduleColumns+" FROM oncall_schedules WHERE organization_id = :organization_id ORDER BY name ASC, id ASC",
		lit.P{"organization_id": organizationId},
	)
}

func (r *oncallScheduleRepository) ListByTeam(tx *sql.Tx, teamId int) ([]*models.OncallSchedule, error) {
	return lit.SelectNamed[models.OncallSchedule](
		tx,
		"SELECT "+oncallScheduleColumns+" FROM oncall_schedules WHERE team_id = :team_id ORDER BY created_at ASC, id ASC",
		lit.P{"team_id": teamId},
	)
}

func (r *oncallScheduleRepository) Create(tx *sql.Tx, schedule *models.OncallSchedule) (int, error) {
	return lit.Insert[models.OncallSchedule](tx, schedule)
}

func (r *oncallScheduleRepository) Update(tx *sql.Tx, schedule *models.OncallSchedule) error {
	return lit.UpdateNamed(tx, schedule, "id = :id", lit.P{"id": schedule.Id})
}

func (r *oncallScheduleRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM oncall_schedules WHERE id = :id", lit.P{"id": id})
}

var OncallScheduleRepository = oncallScheduleRepository{}
