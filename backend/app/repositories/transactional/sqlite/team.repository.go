//go:build !transactional_pg

package sqlite

import (
	"database/sql"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"

	"github.com/google/uuid"
	"github.com/tracewayapp/lit/v2"
)

type teamRepository struct{}

const teamColumns = "id, organization_id, name, description, created_at, updated_at"

func (r *teamRepository) FindById(tx *sql.Tx, id int) (*models.Team, error) {
	return lit.SelectSingleNamed[models.Team](
		tx,
		"SELECT "+teamColumns+" FROM teams WHERE id = :id",
		lit.P{"id": id},
	)
}

func (r *teamRepository) FindByOrganizationAndName(tx *sql.Tx, organizationId int, name string) (*models.Team, error) {
	return lit.SelectSingleNamed[models.Team](
		tx,
		"SELECT "+teamColumns+" FROM teams WHERE organization_id = :organization_id AND LOWER(name) = LOWER(:name)",
		lit.P{"organization_id": organizationId, "name": name},
	)
}

func (r *teamRepository) ListByOrganization(tx *sql.Tx, organizationId int) ([]*models.TeamWithCounts, error) {
	return lit.SelectNamed[models.TeamWithCounts](
		tx,
		`SELECT t.id, t.organization_id, t.name, t.description, t.created_at, t.updated_at,
			(SELECT COUNT(*) FROM team_members tm WHERE tm.team_id = t.id) as member_count,
			(SELECT COUNT(*) FROM project_teams pt WHERE pt.team_id = t.id) as project_count,
			(SELECT COUNT(*) FROM oncall_schedules s WHERE s.team_id = t.id) as schedule_count
		FROM teams t
		WHERE t.organization_id = :organization_id
		ORDER BY t.name ASC, t.id ASC`,
		lit.P{"organization_id": organizationId},
	)
}

func (r *teamRepository) Create(tx *sql.Tx, team *models.Team) (int, error) {
	return lit.Insert[models.Team](tx, team)
}

func (r *teamRepository) Update(tx *sql.Tx, team *models.Team) error {
	return lit.UpdateNamed(tx, team, "id = :id", lit.P{"id": team.Id})
}

func (r *teamRepository) Delete(tx *sql.Tx, id int) error {
	return lit.DeleteNamed(db.Driver, tx, "DELETE FROM teams WHERE id = :id", lit.P{"id": id})
}

func (r *teamRepository) SetMembers(tx *sql.Tx, teamId int, orderedUserIds []int) error {
	if err := lit.DeleteNamed(db.Driver, tx, "DELETE FROM team_members WHERE team_id = :team_id", lit.P{"team_id": teamId}); err != nil {
		return err
	}
	now := time.Now().UTC()
	for position, userId := range orderedUserIds {
		member := &models.TeamMember{
			TeamId:    teamId,
			UserId:    userId,
			Position:  position,
			CreatedAt: now,
		}
		if _, err := lit.Insert[models.TeamMember](tx, member); err != nil {
			return err
		}
	}
	return nil
}

func (r *teamRepository) ListMembersWithUsersByOrganization(tx *sql.Tx, organizationId int) ([]*models.TeamMemberWithUser, error) {
	return lit.SelectNamed[models.TeamMemberWithUser](
		tx,
		`SELECT tm.team_id, tm.user_id, tm.position, u.name, u.email
		FROM team_members tm
		JOIN teams t ON t.id = tm.team_id
		JOIN users u ON u.id = tm.user_id
		WHERE t.organization_id = :organization_id
		ORDER BY tm.team_id ASC, tm.position ASC, tm.id ASC`,
		lit.P{"organization_id": organizationId},
	)
}

func (r *teamRepository) FindMemberUserIds(tx *sql.Tx, teamId int) ([]int, error) {
	members, err := lit.SelectNamed[models.TeamMember](
		tx,
		"SELECT id, team_id, user_id, position, created_at FROM team_members WHERE team_id = :team_id ORDER BY position ASC, id ASC",
		lit.P{"team_id": teamId},
	)
	if err != nil {
		return nil, err
	}
	userIds := make([]int, 0, len(members))
	for _, member := range members {
		userIds = append(userIds, member.UserId)
	}
	return userIds, nil
}

func (r *teamRepository) RemoveUserFromOrgTeams(tx *sql.Tx, organizationId int, userId int) error {
	return lit.DeleteNamed(
		db.Driver,
		tx,
		`DELETE FROM team_members
		WHERE user_id = :user_id
		AND team_id IN (SELECT id FROM teams WHERE organization_id = :organization_id)`,
		lit.P{"user_id": userId, "organization_id": organizationId},
	)
}

func (r *teamRepository) SetProjects(tx *sql.Tx, teamId int, projectIds []uuid.UUID) error {
	if err := lit.DeleteNamed(db.Driver, tx, "DELETE FROM project_teams WHERE team_id = :team_id", lit.P{"team_id": teamId}); err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, projectId := range projectIds {
		link := &models.ProjectTeam{
			ProjectId: projectId,
			TeamId:    teamId,
			CreatedAt: now,
		}
		if _, err := lit.Insert[models.ProjectTeam](tx, link); err != nil {
			return err
		}
	}
	return nil
}

func (r *teamRepository) ListProjectsByOrganization(tx *sql.Tx, organizationId int) ([]*models.TeamProjectRow, error) {
	return lit.SelectNamed[models.TeamProjectRow](
		tx,
		`SELECT pt.team_id, pt.project_id, p.name
		FROM project_teams pt
		JOIN teams t ON t.id = pt.team_id
		JOIN projects p ON p.id = pt.project_id
		WHERE t.organization_id = :organization_id
		ORDER BY pt.team_id ASC, p.name ASC`,
		lit.P{"organization_id": organizationId},
	)
}

func (r *teamRepository) FindProjectTeam(tx *sql.Tx, projectId uuid.UUID) (*models.ProjectTeam, error) {
	return lit.SelectSingleNamed[models.ProjectTeam](
		tx,
		"SELECT id, project_id, team_id, created_at FROM project_teams WHERE project_id = :project_id",
		lit.P{"project_id": projectId},
	)
}

func (r *teamRepository) FindTeamForProject(tx *sql.Tx, projectId uuid.UUID) (*models.Team, error) {
	return lit.SelectSingleNamed[models.Team](
		tx,
		`SELECT t.id, t.organization_id, t.name, t.description, t.created_at, t.updated_at
		FROM teams t
		JOIN project_teams pt ON pt.team_id = t.id
		WHERE pt.project_id = :project_id`,
		lit.P{"project_id": projectId},
	)
}

var TeamRepository = teamRepository{}
