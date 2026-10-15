//go:build transactional_pg

package pg

import (
	"time"

	"github.com/tracewayapp/lit/v2"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"
)

type agentWriteEventRepository struct{}

func (r *agentWriteEventRepository) Insert(ex lit.Executor, e *models.AgentWriteEvent) error {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`INSERT INTO agent_write_events (id, organization_id, project_id, user_id, credential_kind, credential_ref, method, route, path, status_code, created_at)
			VALUES (:id, :organization_id, :project_id, :user_id, :credential_kind, :credential_ref, :method, :route, :path, :status_code, :created_at)`,
		lit.P{
			"id":              e.Id,
			"organization_id": e.OrganizationId,
			"project_id":      e.ProjectId,
			"user_id":         e.UserId,
			"credential_kind": e.CredentialKind,
			"credential_ref":  e.CredentialRef,
			"method":          e.Method,
			"route":           e.Route,
			"path":            e.Path,
			"status_code":     e.StatusCode,
			"created_at":      shared.FormatAuthTime(e.CreatedAt),
		},
	)
	if err != nil {
		return err
	}
	_, err = ex.Exec(query, args...)
	return err
}

func (r *agentWriteEventRepository) ListByOrganization(ex lit.Executor, organizationId, page, pageSize int) ([]*models.AgentWriteEvent, int, error) {
	countQuery, countArgs, err := lit.ParseNamedQuery(
		db.Driver,
		"SELECT COUNT(*) FROM agent_write_events WHERE organization_id = :org_id",
		lit.P{"org_id": organizationId},
	)
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := ex.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		`SELECT e.id, e.organization_id, e.project_id, COALESCE(p.name, ''), e.user_id, COALESCE(u.email, ''),
				e.credential_kind, e.credential_ref, COALESCE(t.name, c.name, ''), e.method, e.route, e.path, e.status_code, e.created_at
			FROM agent_write_events e
			LEFT JOIN projects p ON CAST(p.id AS TEXT) = e.project_id
			LEFT JOIN users u ON u.id = e.user_id
			LEFT JOIN personal_access_tokens t ON t.id = e.credential_ref AND e.credential_kind = 'pat'
			LEFT JOIN oauth_clients c ON c.id = e.credential_ref AND e.credential_kind = 'oauth'
			WHERE e.organization_id = :org_id
			ORDER BY e.created_at DESC
			LIMIT :limit OFFSET :offset`,
		lit.P{"org_id": organizationId, "limit": pageSize, "offset": (page - 1) * pageSize},
	)
	if err != nil {
		return nil, 0, err
	}
	rows, err := ex.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	events := []*models.AgentWriteEvent{}
	for rows.Next() {
		var (
			e         models.AgentWriteEvent
			createdAt string
		)
		if err := rows.Scan(&e.Id, &e.OrganizationId, &e.ProjectId, &e.ProjectName, &e.UserId, &e.UserEmail,
			&e.CredentialKind, &e.CredentialRef, &e.CredentialName, &e.Method, &e.Route, &e.Path, &e.StatusCode, &createdAt); err != nil {
			return nil, 0, err
		}
		if e.CreatedAt, err = shared.ParseAuthTime(createdAt); err != nil {
			return nil, 0, err
		}
		events = append(events, &e)
	}
	return events, total, rows.Err()
}

func (r *agentWriteEventRepository) PruneOlderThan(ex lit.Executor, cutoff time.Time) (int64, error) {
	query, args, err := lit.ParseNamedQuery(
		db.Driver,
		"DELETE FROM agent_write_events WHERE created_at < :cutoff",
		lit.P{"cutoff": shared.FormatAuthTime(cutoff)},
	)
	if err != nil {
		return 0, err
	}
	res, err := ex.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

var AgentWriteEventRepository = agentWriteEventRepository{}
