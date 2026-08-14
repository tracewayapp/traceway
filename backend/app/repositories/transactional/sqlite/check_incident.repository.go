//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type checkIncidentRepository struct{}

const checkIncidentColumns = "id, check_id, project_id, started_at, resolved_at, error_message"

func (r *checkIncidentRepository) Open(tx *sql.Tx, incident *models.CheckIncident) (int, error) {
	return lit.Insert[models.CheckIncident](tx, incident)
}

func (r *checkIncidentRepository) FindOpenByCheck(tx *sql.Tx, checkId int) (*models.CheckIncident, error) {
	return lit.SelectSingleNamed[models.CheckIncident](
		tx,
		"SELECT "+checkIncidentColumns+" FROM check_incidents WHERE check_id = :check_id AND resolved_at IS NULL ORDER BY started_at DESC, id DESC LIMIT 1",
		lit.P{"check_id": checkId},
	)
}

func (r *checkIncidentRepository) Resolve(tx *sql.Tx, id int, resolvedAt time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE check_incidents SET resolved_at = :resolved_at WHERE id = :id AND resolved_at IS NULL",
		lit.P{"resolved_at": resolvedAt.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *checkIncidentRepository) FindRecentByCheck(tx *sql.Tx, checkId int, limit int) ([]*models.CheckIncident, error) {
	return lit.SelectNamed[models.CheckIncident](
		tx,
		"SELECT "+checkIncidentColumns+" FROM check_incidents WHERE check_id = :check_id ORDER BY started_at DESC, id DESC LIMIT :limit",
		lit.P{"check_id": checkId, "limit": limit},
	)
}

func (r *checkIncidentRepository) FindByCheckSince(tx *sql.Tx, checkId int, since time.Time) ([]*models.CheckIncident, error) {
	return lit.SelectNamed[models.CheckIncident](
		tx,
		"SELECT "+checkIncidentColumns+" FROM check_incidents WHERE check_id = :check_id AND (resolved_at IS NULL OR resolved_at >= :since) ORDER BY started_at DESC, id DESC",
		lit.P{"check_id": checkId, "since": since.UTC()},
	)
}

var CheckIncidentRepository = checkIncidentRepository{}
