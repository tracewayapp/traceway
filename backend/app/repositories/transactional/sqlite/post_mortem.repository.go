//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/tracewayapp/lit/v2"
)

type postMortemRepository struct{}

const postMortemColumns = "id, organization_id, project_id, incident_id, title, content_md, tags, created_by, updated_by, created_at, updated_at"

const postMortemUserJoins = " LEFT JOIN users cu ON cu.id = pm.created_by LEFT JOIN users uu ON uu.id = pm.updated_by"

const postMortemDetailColumns = "pm.id, pm.organization_id, pm.project_id, pm.incident_id, pm.title, pm.content_md, pm.tags, pm.created_by, pm.updated_by, pm.created_at, pm.updated_at, cu.name AS created_by_name, uu.name AS updated_by_name"

const postMortemListColumns = "pm.id, pm.organization_id, pm.project_id, pm.incident_id, pm.title, pm.tags, pm.created_by, pm.updated_by, pm.created_at, pm.updated_at, cu.name AS created_by_name, uu.name AS updated_by_name"

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func postMortemFilter(projectId uuid.UUID, search string, tags []string) (string, lit.P) {
	conditions := "pm.project_id = :project_id"
	params := lit.P{"project_id": projectId}
	if search != "" {
		conditions += ` AND (LOWER(pm.title) LIKE :search ESCAPE '\' OR LOWER(pm.content_md) LIKE :search ESCAPE '\' OR LOWER(pm.tags) LIKE :search ESCAPE '\')`
		params["search"] = "%" + likeEscaper.Replace(strings.ToLower(search)) + "%"
	}
	for i, tag := range tags {
		name := fmt.Sprintf("tag_%d", i)
		conditions += " AND pm.tags LIKE :" + name + ` ESCAPE '\'`
		params[name] = "%\"" + likeEscaper.Replace(tag) + "\"%"
	}
	return conditions, params
}

func (r *postMortemRepository) Create(tx *sql.Tx, postMortem *models.PostMortem) (int, error) {
	return lit.Insert[models.PostMortem](tx, postMortem)
}

func (r *postMortemRepository) Update(tx *sql.Tx, postMortem *models.PostMortem) error {
	return lit.UpdateNamed(tx, postMortem, "id = :id AND project_id = :project_id", lit.P{"id": postMortem.Id, "project_id": postMortem.ProjectId})
}

func (r *postMortemRepository) Delete(tx *sql.Tx, id int, projectId uuid.UUID) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM post_mortems WHERE id = :id AND project_id = :project_id", lit.P{"id": id, "project_id": projectId})
}

func (r *postMortemRepository) FindByIdForProject(tx *sql.Tx, id int, projectId uuid.UUID) (*models.PostMortem, error) {
	return lit.SelectSingleNamed[models.PostMortem](
		tx,
		"SELECT "+postMortemColumns+" FROM post_mortems WHERE id = :id AND project_id = :project_id",
		lit.P{"id": id, "project_id": projectId},
	)
}

func (r *postMortemRepository) FindDetailByIdForProject(tx *sql.Tx, id int, projectId uuid.UUID) (*models.PostMortemDetail, error) {
	return lit.SelectSingleNamed[models.PostMortemDetail](
		tx,
		"SELECT "+postMortemDetailColumns+" FROM post_mortems pm"+postMortemUserJoins+" WHERE pm.id = :id AND pm.project_id = :project_id",
		lit.P{"id": id, "project_id": projectId},
	)
}

func (r *postMortemRepository) FindRefsByIncidentIds(tx *sql.Tx, incidentIds []int) ([]*models.PostMortemRef, error) {
	if len(incidentIds) == 0 {
		return []*models.PostMortemRef{}, nil
	}
	inList, params := incidentIdParams(incidentIds)
	return lit.SelectNamed[models.PostMortemRef](
		tx,
		"SELECT id, incident_id FROM post_mortems WHERE incident_id IN ("+inList+")",
		params,
	)
}

func (r *postMortemRepository) ListByProject(tx *sql.Tx, projectId uuid.UUID, search string, tags []string, limit int, offset int) ([]*models.PostMortemListItem, error) {
	conditions, params := postMortemFilter(projectId, search, tags)
	params["limit"] = limit
	params["offset"] = offset
	return lit.SelectNamed[models.PostMortemListItem](
		tx,
		"SELECT "+postMortemListColumns+" FROM post_mortems pm"+postMortemUserJoins+" WHERE "+conditions+" ORDER BY pm.updated_at DESC, pm.id DESC LIMIT :limit OFFSET :offset",
		params,
	)
}

func (r *postMortemRepository) CountByProject(tx *sql.Tx, projectId uuid.UUID, search string, tags []string) (int, error) {
	conditions, params := postMortemFilter(projectId, search, tags)
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM post_mortems pm WHERE "+conditions,
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

func (r *postMortemRepository) RecordEvent(tx *sql.Tx, event *models.PostMortemEvent) error {
	_, err := lit.Insert[models.PostMortemEvent](tx, event)
	return err
}

func (r *postMortemRepository) ListEvents(tx *sql.Tx, postMortemId int, projectId uuid.UUID, limit int) ([]*models.PostMortemEventItem, error) {
	return lit.SelectNamed[models.PostMortemEventItem](
		tx,
		"SELECT e.id, e.post_mortem_id, e.user_id, e.action, e.changes, e.created_at, u.name AS user_name"+
			" FROM post_mortem_events e"+
			" JOIN post_mortems pm ON pm.id = e.post_mortem_id"+
			" LEFT JOIN users u ON u.id = e.user_id"+
			" WHERE e.post_mortem_id = :post_mortem_id AND pm.project_id = :project_id"+
			" ORDER BY e.created_at DESC, e.id DESC LIMIT :limit",
		lit.P{"post_mortem_id": postMortemId, "project_id": projectId, "limit": limit},
	)
}

var PostMortemRepository = postMortemRepository{}
