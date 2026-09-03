//go:build transactional_pg

package pg

import (
	"database/sql"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type checkIncidentRepository struct{}

const checkIncidentColumns = "id, check_id, project_id, status_page_id, title, started_at, resolved_at, error_message"

const checkIncidentJoinedColumns = "i.id, i.check_id, i.project_id, i.status_page_id, i.title, i.started_at, i.resolved_at, i.error_message"

const checkIncidentOrgJoin = "FROM check_incidents i LEFT JOIN synthetic_checks c ON c.id = i.check_id LEFT JOIN projects p ON p.id = c.project_id LEFT JOIN status_pages sp ON sp.id = i.status_page_id"

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

func (r *checkIncidentRepository) FindByIdInOrganization(tx *sql.Tx, id int, organizationId int) (*models.CheckIncident, error) {
	return lit.SelectSingleNamed[models.CheckIncident](
		tx,
		"SELECT "+checkIncidentJoinedColumns+" "+checkIncidentOrgJoin+" WHERE i.id = :id AND (p.organization_id = :organization_id OR sp.organization_id = :organization_id)",
		lit.P{"id": id, "organization_id": organizationId},
	)
}

func (r *checkIncidentRepository) FindByStatusPageSince(tx *sql.Tx, statusPageId int, since time.Time) ([]*models.CheckIncident, error) {
	return lit.SelectNamed[models.CheckIncident](
		tx,
		"SELECT "+checkIncidentColumns+" FROM check_incidents WHERE status_page_id = :status_page_id AND (resolved_at IS NULL OR resolved_at >= :since) ORDER BY started_at DESC, id DESC",
		lit.P{"status_page_id": statusPageId, "since": since.UTC()},
	)
}

func (r *checkIncidentRepository) FindRecentByOrganization(tx *sql.Tx, organizationId int, since time.Time, limit int) ([]*models.OrgIncident, error) {
	return lit.SelectNamed[models.OrgIncident](
		tx,
		"SELECT "+checkIncidentJoinedColumns+", c.name AS check_name, sp.name AS status_page_name "+checkIncidentOrgJoin+" WHERE (p.organization_id = :organization_id OR sp.organization_id = :organization_id) AND (i.resolved_at IS NULL OR i.resolved_at >= :since) ORDER BY i.started_at DESC, i.id DESC LIMIT :limit",
		lit.P{"organization_id": organizationId, "since": since.UTC(), "limit": limit},
	)
}

func statusPageIncidentCondition(statusPageId int, checkIds []int) (string, lit.P) {
	condition := "i.status_page_id = :status_page_id"
	params := lit.P{"status_page_id": statusPageId}
	if len(checkIds) > 0 {
		inList, inParams := incidentIdParams(checkIds)
		for name, value := range inParams {
			params[name] = value
		}
		condition = "(" + condition + " OR i.check_id IN (" + inList + "))"
	}
	return condition, params
}

func (r *checkIncidentRepository) FindByStatusPagePaged(tx *sql.Tx, statusPageId int, checkIds []int, limit int, offset int) ([]*models.OrgIncident, error) {
	condition, params := statusPageIncidentCondition(statusPageId, checkIds)
	params["limit"] = limit
	params["offset"] = offset
	return lit.SelectNamed[models.OrgIncident](
		tx,
		"SELECT "+checkIncidentJoinedColumns+", c.name AS check_name, sp.name AS status_page_name "+checkIncidentOrgJoin+" WHERE "+condition+" ORDER BY i.started_at DESC, i.id DESC LIMIT :limit OFFSET :offset",
		params,
	)
}

func (r *checkIncidentRepository) CountByStatusPage(tx *sql.Tx, statusPageId int, checkIds []int) (int, error) {
	condition, params := statusPageIncidentCondition(statusPageId, checkIds)
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM check_incidents i WHERE "+condition,
		params,
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.Count, nil
}

func (r *checkIncidentRepository) UpdateTitle(tx *sql.Tx, id int, title string) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE check_incidents SET title = :title WHERE id = :id",
		lit.P{"title": title, "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *checkIncidentRepository) UpdateManualTimes(tx *sql.Tx, id int, startedAt time.Time, resolvedAt *time.Time) error {
	var resolved any
	if resolvedAt != nil {
		resolved = resolvedAt.UTC()
	}
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE check_incidents SET started_at = :started_at, resolved_at = :resolved_at WHERE id = :id AND status_page_id IS NOT NULL",
		lit.P{"started_at": startedAt.UTC(), "resolved_at": resolved, "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *checkIncidentRepository) ResolveManual(tx *sql.Tx, id int, resolvedAt time.Time) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"UPDATE check_incidents SET resolved_at = :resolved_at WHERE id = :id AND status_page_id IS NOT NULL AND resolved_at IS NULL",
		lit.P{"resolved_at": resolvedAt.UTC(), "id": id},
	)
	if err != nil {
		return err
	}
	return lit.UpdateNative(tx, query, args...)
}

func (r *checkIncidentRepository) DeleteManual(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM check_incidents WHERE id = :id AND status_page_id IS NOT NULL", lit.P{"id": id})
}

var CheckIncidentRepository = checkIncidentRepository{}

func orgIncidentCondition(organizationId int, search string, from, to *time.Time, params lit.P) string {
	condition := "(p.organization_id = :organization_id OR sp.organization_id = :organization_id)"
	params["organization_id"] = organizationId
	if from != nil {
		condition += " AND (i.resolved_at IS NULL OR i.resolved_at >= :from)"
		params["from"] = from.UTC()
	}
	if to != nil {
		condition += " AND i.started_at <= :to"
		params["to"] = to.UTC()
	}
	if search != "" {
		condition += ` AND (LOWER(i.title) LIKE :search ESCAPE '\' OR LOWER(c.name) LIKE :search ESCAPE '\' OR LOWER(sp.name) LIKE :search ESCAPE '\' OR LOWER(i.error_message) LIKE :search ESCAPE '\')`
		params["search"] = "%" + likeEscaper.Replace(strings.ToLower(search)) + "%"
	}
	return condition
}

func (r *checkIncidentRepository) FindByOrganizationPaged(tx *sql.Tx, organizationId int, search string, from, to *time.Time, limit int, offset int) ([]*models.OrgIncident, error) {
	params := lit.P{"limit": limit, "offset": offset}
	condition := orgIncidentCondition(organizationId, search, from, to, params)
	return lit.SelectNamed[models.OrgIncident](
		tx,
		"SELECT "+checkIncidentJoinedColumns+", c.name AS check_name, sp.name AS status_page_name "+checkIncidentOrgJoin+" WHERE "+condition+" ORDER BY i.started_at DESC, i.id DESC LIMIT :limit OFFSET :offset",
		params,
	)
}

func (r *checkIncidentRepository) CountByOrganizationFiltered(tx *sql.Tx, organizationId int, search string, from, to *time.Time) (int, error) {
	params := lit.P{}
	condition := orgIncidentCondition(organizationId, search, from, to, params)
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count "+checkIncidentOrgJoin+" WHERE "+condition,
		params,
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}
	return result.Count, nil
}
