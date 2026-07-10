//go:build !transactional_pg

package sqlite

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
		"SELECT id, widget_group_id, title, widget_type, config, position, created_at, updated_at FROM widget_group_widgets WHERE widget_group_id = :wg_id ORDER BY position ASC",
		lit.P{"wg_id": widgetGroupId},
	)
}

func (r *widgetGroupRepository) FindWidgetsByGroupWithStar(tx *sql.Tx, widgetGroupId int) ([]*models.WidgetGroupWidgetWithStar, error) {
	return lit.SelectNamed[models.WidgetGroupWidgetWithStar](
		tx,
		`SELECT wgw.id, wgw.widget_group_id, wgw.title, wgw.widget_type, wgw.config, wgw.position,
			(sw.id IS NOT NULL) AS is_starred,
			wgw.created_at, wgw.updated_at
		FROM widget_group_widgets wgw
		LEFT JOIN starred_widgets sw ON sw.widget_id = wgw.id
		WHERE wgw.widget_group_id = :wg_id
		ORDER BY wgw.position ASC`,
		lit.P{"wg_id": widgetGroupId},
	)
}

func (r *widgetGroupRepository) FindWidgetById(tx *sql.Tx, id int) (*models.WidgetGroupWidget, error) {
	return lit.SelectSingleNamed[models.WidgetGroupWidget](
		tx,
		"SELECT id, widget_group_id, title, widget_type, config, position, created_at, updated_at FROM widget_group_widgets WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *widgetGroupRepository) FindStarredWidgetsByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.StarredWidgetWithHome, error) {
	return lit.SelectNamed[models.StarredWidgetWithHome](
		tx,
		`SELECT wgw.id, wgw.widget_group_id, wgw.title, wgw.widget_type, wgw.config, wgw.position,
			sw.position AS home_position, sw.col_span AS home_col_span, sw.size AS home_size,
			wgw.created_at, wgw.updated_at
		FROM starred_widgets sw
		JOIN widget_group_widgets wgw ON wgw.id = sw.widget_id
		JOIN widget_groups wg ON wg.id = wgw.widget_group_id
		WHERE wg.project_id = :project_id
		ORDER BY sw.position ASC, sw.id ASC`,
		lit.P{"project_id": projectId},
	)
}

func (r *widgetGroupRepository) FindStarredByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.StarredWidget, error) {
	return lit.SelectNamed[models.StarredWidget](
		tx,
		`SELECT sw.id, sw.widget_id, sw.position, sw.col_span, sw.size, sw.created_at
		FROM starred_widgets sw
		JOIN widget_group_widgets wgw ON wgw.id = sw.widget_id
		JOIN widget_groups wg ON wg.id = wgw.widget_group_id
		WHERE wg.project_id = :project_id
		ORDER BY sw.position ASC, sw.id ASC`,
		lit.P{"project_id": projectId},
	)
}

func (r *widgetGroupRepository) FindStarredByWidgetId(tx *sql.Tx, projectId uuid.UUID, widgetId int) (*models.StarredWidget, error) {
	return lit.SelectSingleNamed[models.StarredWidget](
		tx,
		`SELECT sw.id, sw.widget_id, sw.position, sw.col_span, sw.size, sw.created_at
		FROM starred_widgets sw
		JOIN widget_group_widgets wgw ON wgw.id = sw.widget_id
		JOIN widget_groups wg ON wg.id = wgw.widget_group_id
		WHERE sw.widget_id = :widget_id AND wg.project_id = :project_id`,
		lit.P{"widget_id": widgetId, "project_id": projectId},
	)
}

func (r *widgetGroupRepository) CreateStarred(tx *sql.Tx, starred *models.StarredWidget) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO starred_widgets (widget_id, position, col_span, size, created_at)
		VALUES (:widget_id, :position, :col_span, :size, :created_at)
		ON CONFLICT (widget_id) DO NOTHING`,
		lit.P{
			"widget_id":  starred.WidgetId,
			"position":   starred.Position,
			"col_span":   starred.ColSpan,
			"size":       starred.Size,
			"created_at": starred.CreatedAt,
		},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *widgetGroupRepository) UpdateStarred(tx *sql.Tx, starred *models.StarredWidget) error {
	return lit.UpdateNamed(tx, starred, "id = :id", lit.P{"id": starred.Id})
}

func (r *widgetGroupRepository) DeleteStarredByWidgetId(tx *sql.Tx, widgetId int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM starred_widgets WHERE widget_id = :widget_id", lit.P{"widget_id": widgetId})
}

func (r *widgetGroupRepository) DeleteStarredByGroup(tx *sql.Tx, widgetGroupId int) error {
	return lit.DeleteNamed(
		db.Driver, tx,
		"DELETE FROM starred_widgets WHERE widget_id IN (SELECT id FROM widget_group_widgets WHERE widget_group_id = :wg_id)",
		lit.P{"wg_id": widgetGroupId},
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
