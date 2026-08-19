//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type incidentUpdateRepository struct{}

const incidentUpdateColumns = "id, incident_id, status, message, created_by, created_at"

func incidentIdParams(incidentIds []int) (string, lit.P) {
	params := lit.P{}
	names := make([]string, len(incidentIds))
	for i, id := range incidentIds {
		name := fmt.Sprintf("id_%d", i)
		names[i] = ":" + name
		params[name] = id
	}
	return strings.Join(names, ", "), params
}

func (r *incidentUpdateRepository) Create(tx *sql.Tx, update *models.IncidentUpdate) (int, error) {
	return lit.Insert[models.IncidentUpdate](tx, update)
}

func (r *incidentUpdateRepository) FindByIncident(tx *sql.Tx, incidentId int) ([]*models.IncidentUpdate, error) {
	return lit.SelectNamed[models.IncidentUpdate](
		tx,
		"SELECT "+incidentUpdateColumns+" FROM incident_updates WHERE incident_id = :incident_id ORDER BY created_at DESC, id DESC",
		lit.P{"incident_id": incidentId},
	)
}

func (r *incidentUpdateRepository) FindById(tx *sql.Tx, id int) (*models.IncidentUpdate, error) {
	return lit.SelectSingleNamed[models.IncidentUpdate](
		tx,
		"SELECT "+incidentUpdateColumns+" FROM incident_updates WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *incidentUpdateRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM incident_updates WHERE id = :id", lit.P{"id": id})
}

func (r *incidentUpdateRepository) FindByIncidentIds(tx *sql.Tx, incidentIds []int) ([]*models.IncidentUpdate, error) {
	if len(incidentIds) == 0 {
		return []*models.IncidentUpdate{}, nil
	}
	inList, params := incidentIdParams(incidentIds)
	return lit.SelectNamed[models.IncidentUpdate](
		tx,
		"SELECT "+incidentUpdateColumns+" FROM incident_updates WHERE incident_id IN ("+inList+") ORDER BY incident_id ASC, created_at DESC, id DESC",
		params,
	)
}

func (r *incidentUpdateRepository) CountByIncidentIds(tx *sql.Tx, incidentIds []int) (map[int]int, error) {
	if len(incidentIds) == 0 {
		return map[int]int{}, nil
	}
	inList, params := incidentIdParams(incidentIds)
	rows, err := lit.SelectNamed[models.IncidentUpdateCount](
		tx,
		"SELECT incident_id, COUNT(*) AS count FROM incident_updates WHERE incident_id IN ("+inList+") GROUP BY incident_id",
		params,
	)
	if err != nil {
		return nil, err
	}
	counts := make(map[int]int, len(rows))
	for _, row := range rows {
		counts[row.IncidentId] = row.Count
	}
	return counts, nil
}

var IncidentUpdateRepository = incidentUpdateRepository{}
