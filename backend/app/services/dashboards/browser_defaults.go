package dashboards

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/sdkconfig"
)

// EnsureBrowserDefault runs in the project creation transaction so a project is
// never visible without its dashboard. The organization lock serializes names
// with other concurrently created projects on PostgreSQL.
func EnsureBrowserDefault(tx *sql.Tx, project *models.Project) error {
	if project.OrganizationId == nil || !sdkconfig.IsBrowserFramework(project.Framework) {
		return nil
	}
	org := *project.OrganizationId
	if !db.IsSQLite() {
		if _, err := tx.Exec("SELECT pg_advisory_xact_lock(824738, $1)", org); err != nil {
			return err
		}
	}
	existing, err := lit.SelectNamed[models.Dashboard](tx, `SELECT d.* FROM dashboards d JOIN project_dashboards pd ON pd.dashboard_id = d.id WHERE pd.project_id = :id AND d.template_key = :key`, lit.P{"id": project.Id, "key": "browser-performance"})
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	template, err := lit.SelectSingleNamed[models.DashboardTemplate](tx, "SELECT * FROM dashboard_templates WHERE key = :key", lit.P{"key": "browser-performance"})
	if err != nil {
		return err
	}
	if template == nil {
		return fmt.Errorf("browser-performance dashboard template is not seeded")
	}
	def, err := ParseDefinition(template.Definition)
	if err != nil {
		return err
	}
	EnsureWidgetIds(def)
	definition, err := MarshalDefinition(def)
	if err != nil {
		return err
	}
	dashboards, err := lit.SelectNamed[models.Dashboard](tx, "SELECT * FROM dashboards WHERE organization_id = :org", lit.P{"org": org})
	if err != nil {
		return err
	}
	used := make(map[string]bool, len(dashboards))
	for _, d := range dashboards {
		used[strings.ToLower(d.Name)] = true
	}
	base := TruncateName(template.Name, 100)
	name := base
	for n := 2; used[strings.ToLower(name)]; n++ {
		suffix := fmt.Sprintf(" %d", n)
		name = TruncateName(base, 100-len(suffix)) + suffix
	}
	now := time.Now().UTC()
	dashboard := &models.Dashboard{OrganizationId: org, Name: name, Description: template.Description, Definition: definition, TemplateKey: &template.Key, CreatedAt: now, UpdatedAt: now}
	id, err := lit.Insert(tx, dashboard)
	if err != nil {
		return err
	}
	position, err := lit.SelectSingleNamed[models.CountResult](tx, "SELECT COALESCE(MAX(position), -1) + 1 AS count FROM project_dashboards WHERE project_id = :id", lit.P{"id": project.Id})
	if err != nil {
		return err
	}
	_, err = lit.Insert(tx, &models.ProjectDashboard{ProjectId: project.Id, DashboardId: id, Position: position.Count, CreatedAt: now})
	return err
}
