//go:build transactional_pg

package pg

import (
	"database/sql"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type projectTelemetrySourceRepository struct{}

const projectTelemetrySourceColumns = "id, project_id, domain, integration_id, priority"

func (r *projectTelemetrySourceRepository) FindByProject(tx *sql.Tx, projectId uuid.UUID) ([]*models.ProjectTelemetrySource, error) {
	return lit.SelectNamed[models.ProjectTelemetrySource](
		tx,
		"SELECT "+projectTelemetrySourceColumns+" FROM project_telemetry_sources WHERE project_id = :project_id ORDER BY domain ASC, priority ASC, id ASC",
		lit.P{"project_id": projectId},
	)
}

// Replace sets the ordered integrations answering one domain for a project;
// their position is their priority.
func (r *projectTelemetrySourceRepository) Replace(tx *sql.Tx, projectId uuid.UUID, domain string, integrationIds []int) error {
	if err := lit.DeleteNamed(db.Driver, tx, "DELETE FROM project_telemetry_sources WHERE project_id = :project_id AND domain = :domain", lit.P{"project_id": projectId, "domain": domain}); err != nil {
		return err
	}
	for priority, integrationId := range integrationIds {
		row := &models.ProjectTelemetrySource{ProjectId: projectId, Domain: domain, IntegrationId: integrationId, Priority: priority}
		if _, err := lit.Insert[models.ProjectTelemetrySource](tx, row); err != nil {
			return err
		}
	}
	return nil
}

var ProjectTelemetrySourceRepository = projectTelemetrySourceRepository{}
