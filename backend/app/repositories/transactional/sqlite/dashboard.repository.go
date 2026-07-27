//go:build !transactional_pg

package sqlite

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type dashboardRepository struct{}

const dashboardColumns = "id, organization_id, name, description, definition, template_key, created_by, created_at, updated_at"

func (r *dashboardRepository) FindById(tx *sql.Tx, id int) (*models.Dashboard, error) {
	return lit.SelectSingleNamed[models.Dashboard](
		tx,
		"SELECT "+dashboardColumns+" FROM dashboards WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *dashboardRepository) FindByOrganization(tx *sql.Tx, organizationId int) ([]*models.Dashboard, error) {
	return lit.SelectNamed[models.Dashboard](
		tx,
		"SELECT "+dashboardColumns+" FROM dashboards WHERE organization_id = :organization_id ORDER BY name ASC, id ASC",
		lit.P{"organization_id": organizationId},
	)
}

func (r *dashboardRepository) FindByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.Dashboard, error) {
	return lit.SelectNamed[models.Dashboard](
		tx,
		`SELECT d.id, d.organization_id, d.name, d.description, d.definition, d.template_key, d.created_by, d.created_at, d.updated_at
		FROM dashboards d
		JOIN project_dashboards pd ON pd.dashboard_id = d.id
		WHERE pd.project_id = :project_id
		ORDER BY pd.position ASC, pd.id ASC`,
		lit.P{"project_id": projectId},
	)
}

func (r *dashboardRepository) FindStarredDashboardsByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.Dashboard, error) {
	return lit.SelectNamed[models.Dashboard](
		tx,
		"SELECT "+dashboardColumns+" FROM dashboards WHERE id IN (SELECT dashboard_id FROM starred_dashboard_widgets WHERE project_id = :project_id)",
		lit.P{"project_id": projectId},
	)
}

func (r *dashboardRepository) Create(tx *sql.Tx, dashboard *models.Dashboard) (int, error) {
	return lit.Insert[models.Dashboard](tx, dashboard)
}

func (r *dashboardRepository) Update(tx *sql.Tx, dashboard *models.Dashboard) error {
	return lit.UpdateNamed(tx, dashboard, "id = :id", lit.P{"id": dashboard.Id})
}

func (r *dashboardRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM dashboards WHERE id = :id", lit.P{"id": id})
}

func (r *dashboardRepository) FindAssignmentsByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.ProjectDashboard, error) {
	return lit.SelectNamed[models.ProjectDashboard](
		tx,
		"SELECT id, project_id, dashboard_id, position, created_at FROM project_dashboards WHERE project_id = :project_id ORDER BY position ASC, id ASC",
		lit.P{"project_id": projectId},
	)
}

func (r *dashboardRepository) FindAssignmentsByDashboard(tx *sql.Tx, dashboardId int) ([]*models.ProjectDashboard, error) {
	return lit.SelectNamed[models.ProjectDashboard](
		tx,
		"SELECT id, project_id, dashboard_id, position, created_at FROM project_dashboards WHERE dashboard_id = :dashboard_id ORDER BY id ASC",
		lit.P{"dashboard_id": dashboardId},
	)
}

func (r *dashboardRepository) FindAssignmentsByOrganization(tx *sql.Tx, organizationId int) ([]*models.DashboardAssignment, error) {
	return lit.SelectNamed[models.DashboardAssignment](
		tx,
		`SELECT pd.dashboard_id, pd.project_id
		FROM project_dashboards pd
		JOIN dashboards d ON d.id = pd.dashboard_id
		WHERE d.organization_id = :organization_id`,
		lit.P{"organization_id": organizationId},
	)
}

func (r *dashboardRepository) FindAssignment(tx *sql.Tx, projectId uuid.UUID, dashboardId int) (*models.ProjectDashboard, error) {
	return lit.SelectSingleNamed[models.ProjectDashboard](
		tx,
		"SELECT id, project_id, dashboard_id, position, created_at FROM project_dashboards WHERE project_id = :project_id AND dashboard_id = :dashboard_id",
		lit.P{"project_id": projectId, "dashboard_id": dashboardId},
	)
}

func (r *dashboardRepository) CreateAssignment(tx *sql.Tx, assignment *models.ProjectDashboard) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO project_dashboards (project_id, dashboard_id, position, created_at)
		VALUES (:project_id, :dashboard_id, :position, :created_at)
		ON CONFLICT (project_id, dashboard_id) DO NOTHING`,
		lit.P{
			"project_id":   assignment.ProjectId,
			"dashboard_id": assignment.DashboardId,
			"position":     assignment.Position,
			"created_at":   assignment.CreatedAt,
		},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *dashboardRepository) UpdateAssignment(tx *sql.Tx, assignment *models.ProjectDashboard) error {
	return lit.UpdateNamed(tx, assignment, "id = :id", lit.P{"id": assignment.Id})
}

func (r *dashboardRepository) DeleteAssignment(tx *sql.Tx, projectId uuid.UUID, dashboardId int) error {
	return lit.DeleteNamed(
		db.Driver, tx,
		"DELETE FROM project_dashboards WHERE project_id = :project_id AND dashboard_id = :dashboard_id",
		lit.P{"project_id": projectId, "dashboard_id": dashboardId},
	)
}

func (r *dashboardRepository) DeleteAssignmentsByDashboard(tx *sql.Tx, dashboardId int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM project_dashboards WHERE dashboard_id = :dashboard_id", lit.P{"dashboard_id": dashboardId})
}

func (r *dashboardRepository) FindStarredByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.StarredDashboardWidget, error) {
	return lit.SelectNamed[models.StarredDashboardWidget](
		tx,
		`SELECT s.id, s.project_id, s.dashboard_id, s.widget_id, s.position, s.col_span, s.size, s.created_at
		FROM starred_dashboard_widgets s
		JOIN project_dashboards pd ON pd.dashboard_id = s.dashboard_id AND pd.project_id = s.project_id
		WHERE s.project_id = :project_id
		ORDER BY s.position ASC, s.id ASC`,
		lit.P{"project_id": projectId},
	)
}

func (r *dashboardRepository) FindStarredByProjectAndDashboard(tx *sql.Tx, projectId uuid.UUID, dashboardId int) ([]*models.StarredDashboardWidget, error) {
	return lit.SelectNamed[models.StarredDashboardWidget](
		tx,
		"SELECT id, project_id, dashboard_id, widget_id, position, col_span, size, created_at FROM starred_dashboard_widgets WHERE project_id = :project_id AND dashboard_id = :dashboard_id ORDER BY position ASC, id ASC",
		lit.P{"project_id": projectId, "dashboard_id": dashboardId},
	)
}

func (r *dashboardRepository) FindStarredById(tx *sql.Tx, projectId uuid.UUID, id int) (*models.StarredDashboardWidget, error) {
	return lit.SelectSingleNamed[models.StarredDashboardWidget](
		tx,
		"SELECT id, project_id, dashboard_id, widget_id, position, col_span, size, created_at FROM starred_dashboard_widgets WHERE id = :id AND project_id = :project_id",
		lit.P{"id": id, "project_id": projectId},
	)
}

func (r *dashboardRepository) FindStarred(tx *sql.Tx, projectId uuid.UUID, dashboardId int, widgetId string) (*models.StarredDashboardWidget, error) {
	return lit.SelectSingleNamed[models.StarredDashboardWidget](
		tx,
		"SELECT id, project_id, dashboard_id, widget_id, position, col_span, size, created_at FROM starred_dashboard_widgets WHERE project_id = :project_id AND dashboard_id = :dashboard_id AND widget_id = :widget_id",
		lit.P{"project_id": projectId, "dashboard_id": dashboardId, "widget_id": widgetId},
	)
}

func (r *dashboardRepository) CreateStarred(tx *sql.Tx, starred *models.StarredDashboardWidget) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO starred_dashboard_widgets (project_id, dashboard_id, widget_id, position, col_span, size, created_at)
		VALUES (:project_id, :dashboard_id, :widget_id, :position, :col_span, :size, :created_at)
		ON CONFLICT (project_id, dashboard_id, widget_id) DO NOTHING`,
		lit.P{
			"project_id":   starred.ProjectId,
			"dashboard_id": starred.DashboardId,
			"widget_id":    starred.WidgetId,
			"position":     starred.Position,
			"col_span":     starred.ColSpan,
			"size":         starred.Size,
			"created_at":   starred.CreatedAt,
		},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *dashboardRepository) UpdateStarred(tx *sql.Tx, starred *models.StarredDashboardWidget) error {
	return lit.UpdateNamed(tx, starred, "id = :id", lit.P{"id": starred.Id})
}

func (r *dashboardRepository) DeleteStarred(tx *sql.Tx, projectId uuid.UUID, dashboardId int, widgetId string) error {
	return lit.DeleteNamed(
		db.Driver, tx,
		"DELETE FROM starred_dashboard_widgets WHERE project_id = :project_id AND dashboard_id = :dashboard_id AND widget_id = :widget_id",
		lit.P{"project_id": projectId, "dashboard_id": dashboardId, "widget_id": widgetId},
	)
}

func (r *dashboardRepository) DeleteStarredByProjectAndDashboard(tx *sql.Tx, projectId uuid.UUID, dashboardId int) error {
	return lit.DeleteNamed(
		db.Driver, tx,
		"DELETE FROM starred_dashboard_widgets WHERE project_id = :project_id AND dashboard_id = :dashboard_id",
		lit.P{"project_id": projectId, "dashboard_id": dashboardId},
	)
}

func (r *dashboardRepository) DeleteStarredByDashboard(tx *sql.Tx, dashboardId int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM starred_dashboard_widgets WHERE dashboard_id = :dashboard_id", lit.P{"dashboard_id": dashboardId})
}

func (r *dashboardRepository) DeleteStarredByWidget(tx *sql.Tx, dashboardId int, widgetId string) error {
	return lit.DeleteNamed(
		db.Driver, tx,
		"DELETE FROM starred_dashboard_widgets WHERE dashboard_id = :dashboard_id AND widget_id = :widget_id",
		lit.P{"dashboard_id": dashboardId, "widget_id": widgetId},
	)
}

var DashboardRepository = dashboardRepository{}
