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

type postMortemRepository struct{}

const postMortemColumns = "id, organization_id, incident_id, title, content_md, tags, created_by, created_at, updated_at"

const postMortemListColumns = "id, organization_id, incident_id, title, tags, created_by, created_at, updated_at"

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func postMortemFilter(organizationId int, search string, tags []string) (string, lit.P) {
	conditions := "organization_id = :organization_id"
	params := lit.P{"organization_id": organizationId}
	if search != "" {
		conditions += ` AND (LOWER(title) LIKE :search ESCAPE '\' OR LOWER(content_md) LIKE :search ESCAPE '\' OR LOWER(tags) LIKE :search ESCAPE '\')`
		params["search"] = "%" + likeEscaper.Replace(strings.ToLower(search)) + "%"
	}
	for i, tag := range tags {
		name := fmt.Sprintf("tag_%d", i)
		conditions += " AND tags LIKE :" + name + ` ESCAPE '\'`
		params[name] = "%\"" + likeEscaper.Replace(tag) + "\"%"
	}
	return conditions, params
}

func (r *postMortemRepository) Create(tx *sql.Tx, postMortem *models.PostMortem) (int, error) {
	return lit.Insert[models.PostMortem](tx, postMortem)
}

func (r *postMortemRepository) Update(tx *sql.Tx, postMortem *models.PostMortem) error {
	return lit.UpdateNamed(tx, postMortem, "id = :id AND organization_id = :organization_id", lit.P{"id": postMortem.Id, "organization_id": postMortem.OrganizationId})
}

func (r *postMortemRepository) Delete(tx *sql.Tx, id int, organizationId int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM post_mortems WHERE id = :id AND organization_id = :organization_id", lit.P{"id": id, "organization_id": organizationId})
}

func (r *postMortemRepository) FindByIdForOrganization(tx *sql.Tx, id int, organizationId int) (*models.PostMortem, error) {
	return lit.SelectSingleNamed[models.PostMortem](
		tx,
		"SELECT "+postMortemColumns+" FROM post_mortems WHERE id = :id AND organization_id = :organization_id",
		lit.P{"id": id, "organization_id": organizationId},
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

func (r *postMortemRepository) ListByOrganization(tx *sql.Tx, organizationId int, search string, tags []string, limit int, offset int) ([]*models.PostMortemListItem, error) {
	conditions, params := postMortemFilter(organizationId, search, tags)
	params["limit"] = limit
	params["offset"] = offset
	return lit.SelectNamed[models.PostMortemListItem](
		tx,
		"SELECT "+postMortemListColumns+" FROM post_mortems WHERE "+conditions+" ORDER BY updated_at DESC, id DESC LIMIT :limit OFFSET :offset",
		params,
	)
}

func (r *postMortemRepository) CountByOrganization(tx *sql.Tx, organizationId int, search string, tags []string) (int, error) {
	conditions, params := postMortemFilter(organizationId, search, tags)
	result, err := lit.SelectSingleNamed[models.CountResult](
		tx,
		"SELECT COUNT(*) as count FROM post_mortems WHERE "+conditions,
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

var PostMortemRepository = postMortemRepository{}
