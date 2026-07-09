//go:build transactional_pg

package pg

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type widgetGroupRepository struct{}

func (r *widgetGroupRepository) FindByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.WidgetGroup, error) {
	return lit.SelectNamed[models.WidgetGroup](
		tx,
		"SELECT id, project_id, name, description, is_default, created_by, created_at, updated_at FROM widget_groups WHERE project_id = :project_id ORDER BY is_default DESC, name ASC",
		lit.P{"project_id": projectId},
	)
}

func (r *widgetGroupRepository) FindById(tx *sql.Tx, id int) (*models.WidgetGroup, error) {
	return lit.SelectSingleNamed[models.WidgetGroup](
		tx,
		"SELECT id, project_id, name, description, is_default, created_by, created_at, updated_at FROM widget_groups WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *widgetGroupRepository) Create(tx *sql.Tx, group *models.WidgetGroup) (int, error) {
	return lit.Insert[models.WidgetGroup](tx, group)
}

func (r *widgetGroupRepository) Update(tx *sql.Tx, group *models.WidgetGroup) error {
	return lit.UpdateNamed(tx, group, "id = :id", lit.P{"id": group.Id})
}

func (r *widgetGroupRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM widget_groups WHERE id = :id", lit.P{"id": id})
}

func (r *widgetGroupRepository) FindWidgetsByGroup(tx *sql.Tx, widgetGroupId int) ([]*models.WidgetGroupWidget, error) {
	return lit.SelectNamed[models.WidgetGroupWidget](
		tx,
		"SELECT id, widget_group_id, title, widget_type, config, position, is_starred, created_at, updated_at FROM widget_group_widgets WHERE widget_group_id = :wg_id ORDER BY position ASC",
		lit.P{"wg_id": widgetGroupId},
	)
}

func (r *widgetGroupRepository) FindWidgetById(tx *sql.Tx, id int) (*models.WidgetGroupWidget, error) {
	return lit.SelectSingleNamed[models.WidgetGroupWidget](
		tx,
		"SELECT id, widget_group_id, title, widget_type, config, position, is_starred, created_at, updated_at FROM widget_group_widgets WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *widgetGroupRepository) FindStarredWidgetsByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.WidgetGroupWidget, error) {
	return lit.SelectNamed[models.WidgetGroupWidget](
		tx,
		`SELECT wgw.id, wgw.widget_group_id, wgw.title, wgw.widget_type, wgw.config, wgw.position, wgw.is_starred, wgw.created_at, wgw.updated_at
		FROM widget_group_widgets wgw
		JOIN widget_groups wg ON wg.id = wgw.widget_group_id
		WHERE wg.project_id = :project_id AND wgw.is_starred = :starred
		ORDER BY wgw.updated_at DESC`,
		lit.P{"project_id": projectId, "starred": true},
	)
}

func (r *widgetGroupRepository) CreateWidget(tx *sql.Tx, widget *models.WidgetGroupWidget) (int, error) {
	return lit.Insert[models.WidgetGroupWidget](tx, widget)
}

func (r *widgetGroupRepository) UpdateWidget(tx *sql.Tx, widget *models.WidgetGroupWidget) error {
	return lit.UpdateNamed(tx, widget, "id = :id", lit.P{"id": widget.Id})
}

func (r *widgetGroupRepository) DeleteWidget(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM widget_group_widgets WHERE id = :id", lit.P{"id": id})
}

// DeleteWidgetsByGroup removes every widget belonging to the group. Use this
// before WidgetGroupRepository.Delete so the group + child widgets disappear
// together within the same transaction (rather than relying on the FK cascade
// — explicit is cheaper to audit and makes the SQL log self-explanatory).
func (r *widgetGroupRepository) DeleteWidgetsByGroup(tx *sql.Tx, widgetGroupId int) error {
	return lit.DeleteNamed(
		db.Driver, tx,
		"DELETE FROM widget_group_widgets WHERE widget_group_id = :wg_id",
		lit.P{"wg_id": widgetGroupId},
	)
}

var WidgetGroupRepository = widgetGroupRepository{}
