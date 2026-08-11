package controllers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type teamController struct{}

var TeamController = teamController{}

type teamResponse struct {
	*models.TeamWithCounts
	Members  []*models.TeamMemberWithUser `json:"members"`
	Projects []teamProjectResponse        `json:"projects"`
}

type teamProjectResponse struct {
	ProjectId uuid.UUID `json:"projectId"`
	Name      string    `json:"name"`
}

type createTeamRequest struct {
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	MemberUserIds []int       `json:"memberUserIds"`
	ProjectIds    []uuid.UUID `json:"projectIds"`
}

// MemberUserIds and ProjectIds are optional; when present the whole edit
// applies in one transaction.
type updateTeamRequest struct {
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	MemberUserIds *[]int       `json:"memberUserIds"`
	ProjectIds    *[]uuid.UUID `json:"projectIds"`
}

type setTeamMembersRequest struct {
	UserIds []int `json:"userIds"`
}

type setTeamProjectsRequest struct {
	ProjectIds []uuid.UUID `json:"projectIds"`
}

func (c *teamController) List(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	teams, err := transactional.TeamRepository.ListByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list teams: %w", err))
		return
	}
	members, err := transactional.TeamRepository.ListMembersWithUsersByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list team members: %w", err))
		return
	}
	projects, err := transactional.TeamRepository.ListProjectsByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list team projects: %w", err))
		return
	}

	membersByTeam := map[int][]*models.TeamMemberWithUser{}
	for _, member := range members {
		membersByTeam[member.TeamId] = append(membersByTeam[member.TeamId], member)
	}
	projectsByTeam := map[int][]teamProjectResponse{}
	for _, project := range projects {
		projectsByTeam[project.TeamId] = append(projectsByTeam[project.TeamId], teamProjectResponse{ProjectId: project.ProjectId, Name: project.Name})
	}

	response := make([]teamResponse, 0, len(teams))
	for _, team := range teams {
		entry := teamResponse{TeamWithCounts: team, Members: membersByTeam[team.Id], Projects: projectsByTeam[team.Id]}
		if entry.Members == nil {
			entry.Members = []*models.TeamMemberWithUser{}
		}
		if entry.Projects == nil {
			entry.Projects = []teamProjectResponse{}
		}
		response = append(response, entry)
	}
	ctx.JSON(http.StatusOK, gin.H{"teams": response})
}

func (c *teamController) Create(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	var request createTeamRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if message := validateTeamName(request.Name); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	existing, err := transactional.TeamRepository.FindByOrganizationAndName(tx, organizationId, request.Name)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check team name: %w", err))
		return
	}
	if existing != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A team with this name already exists."})
		return
	}
	if message, err := c.checkMembersInOrg(tx, organizationId, request.MemberUserIds); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate team members: %w", err))
		return
	} else if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}

	now := time.Now().UTC()
	team := &models.Team{
		OrganizationId: organizationId,
		Name:           request.Name,
		Description:    request.Description,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	teamId, err := transactional.TeamRepository.Create(tx, team)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create team: %w", err))
		return
	}
	team.Id = teamId

	if len(request.MemberUserIds) > 0 {
		if err := transactional.TeamRepository.SetMembers(tx, teamId, request.MemberUserIds); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to set team members: %w", err))
			return
		}
	}
	if len(request.ProjectIds) > 0 {
		if message, err := c.checkProjectsAssignable(tx, organizationId, teamId, request.ProjectIds); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate team projects: %w", err))
			return
		} else if message != "" {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
			return
		}
		if err := transactional.TeamRepository.SetProjects(tx, teamId, request.ProjectIds); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to set team projects: %w", err))
			return
		}
	}
	ctx.JSON(http.StatusCreated, team)
}

func (c *teamController) Update(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	team, ok := c.loadTeam(ctx, organizationId)
	if !ok {
		return
	}
	var request updateTeamRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if message := validateTeamName(request.Name); message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	existing, err := transactional.TeamRepository.FindByOrganizationAndName(tx, organizationId, request.Name)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check team name: %w", err))
		return
	}
	if existing != nil && existing.Id != team.Id {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A team with this name already exists."})
		return
	}

	if request.MemberUserIds != nil {
		if message, err := c.checkMembersInOrg(tx, organizationId, *request.MemberUserIds); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate team members: %w", err))
			return
		} else if message != "" {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
			return
		}
	}
	if request.ProjectIds != nil {
		if message, err := c.checkProjectsAssignable(tx, organizationId, team.Id, *request.ProjectIds); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate team projects: %w", err))
			return
		} else if message != "" {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
			return
		}
	}

	team.Name = request.Name
	team.Description = request.Description
	team.UpdatedAt = time.Now().UTC()
	if err := transactional.TeamRepository.Update(tx, team); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update team: %w", err))
		return
	}
	if request.MemberUserIds != nil {
		if err := transactional.TeamRepository.SetMembers(tx, team.Id, *request.MemberUserIds); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to set team members: %w", err))
			return
		}
	}
	if request.ProjectIds != nil {
		if err := transactional.TeamRepository.SetProjects(tx, team.Id, *request.ProjectIds); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to set team projects: %w", err))
			return
		}
	}
	ctx.JSON(http.StatusOK, team)
}

func (c *teamController) Delete(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	team, ok := c.loadTeam(ctx, organizationId)
	if !ok {
		return
	}

	message, err := c.checkPolicyReferences(tx, organizationId, team.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check referencing policies: %w", err))
		return
	}
	if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}

	if err := transactional.TeamRepository.Delete(tx, team.Id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete team: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Team deleted"})
}

func (c *teamController) SetMembers(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	team, ok := c.loadTeam(ctx, organizationId)
	if !ok {
		return
	}
	var request setTeamMembersRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if message, err := c.checkMembersInOrg(tx, organizationId, request.UserIds); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate team members: %w", err))
		return
	} else if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	if err := transactional.TeamRepository.SetMembers(tx, team.Id, request.UserIds); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to set team members: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Members updated"})
}

func (c *teamController) SetProjects(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	team, ok := c.loadTeam(ctx, organizationId)
	if !ok {
		return
	}
	var request setTeamProjectsRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if message, err := c.checkProjectsAssignable(tx, organizationId, team.Id, request.ProjectIds); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate team projects: %w", err))
		return
	} else if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	if err := transactional.TeamRepository.SetProjects(tx, team.Id, request.ProjectIds); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to set team projects: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Projects updated"})
}

func (c *teamController) loadTeam(ctx *gin.Context, organizationId int) (*models.Team, bool) {
	tx := db.GetTx(ctx)
	teamId, err := strconv.Atoi(ctx.Param("teamId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return nil, false
	}
	team, err := transactional.TeamRepository.FindById(tx, teamId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load team: %w", err))
		return nil, false
	}
	if team == nil || team.OrganizationId != organizationId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return nil, false
	}
	return team, true
}

// checkPolicyReferences returns a user-facing message when an escalation policy
// targets the team or one of its schedules.
func (c *teamController) checkPolicyReferences(tx *sql.Tx, organizationId int, teamId int) (string, error) {
	referencing, err := oncall.PoliciesReferencing(tx, organizationId, oncall.TargetTeam, teamId)
	if err != nil {
		return "", err
	}
	if len(referencing) > 0 {
		return "This team is used by escalation policy(ies): " + strings.Join(referencing, ", ") + ". Remove those steps first.", nil
	}

	schedules, err := transactional.OncallScheduleRepository.ListByTeam(tx, teamId)
	if err != nil {
		return "", err
	}
	scheduleIds := make([]int, 0, len(schedules))
	for _, schedule := range schedules {
		scheduleIds = append(scheduleIds, schedule.Id)
	}
	referencing, err = oncall.PoliciesReferencing(tx, organizationId, oncall.TargetSchedule, scheduleIds...)
	if err != nil {
		return "", err
	}
	if len(referencing) > 0 {
		return "Deleting this team would delete schedules used by escalation policy(ies): " + strings.Join(referencing, ", ") + ". Remove those steps first.", nil
	}
	return "", nil
}

// checkMembersInOrg returns a user-facing message when a userId is not a
// member of the organization or appears twice.
func (c *teamController) checkMembersInOrg(tx *sql.Tx, organizationId int, userIds []int) (string, error) {
	members, err := transactional.OrganizationRepository.GetMembersWithDetails(tx, organizationId)
	if err != nil {
		return "", err
	}
	memberSet := make(map[int]bool, len(members))
	for _, member := range members {
		memberSet[member.Id] = true
	}
	seen := map[int]bool{}
	for _, userId := range userIds {
		if !memberSet[userId] {
			return "Every team member must be a member of the organization.", nil
		}
		if seen[userId] {
			return "The same member is listed twice.", nil
		}
		seen[userId] = true
	}
	return "", nil
}

// checkProjectsAssignable enforces org membership of each project and the
// one-owning-team-per-project rule.
func (c *teamController) checkProjectsAssignable(tx *sql.Tx, organizationId int, teamId int, projectIds []uuid.UUID) (string, error) {
	seen := map[uuid.UUID]bool{}
	for _, projectId := range projectIds {
		if seen[projectId] {
			return "The same project is listed twice.", nil
		}
		seen[projectId] = true
		project, err := transactional.ProjectRepository.FindById(tx, projectId)
		if err != nil {
			return "", err
		}
		if project == nil || project.OrganizationId == nil || *project.OrganizationId != organizationId {
			return "Every project must belong to this organization.", nil
		}
		owner, err := transactional.TeamRepository.FindProjectTeam(tx, projectId)
		if err != nil {
			return "", err
		}
		if owner != nil && owner.TeamId != teamId {
			return "The project \"" + project.Name + "\" is already owned by another team.", nil
		}
	}
	return "", nil
}

func validateTeamName(name string) string {
	if name == "" {
		return "A team name is required."
	}
	if len(name) > 100 {
		return "The team name can be at most 100 characters."
	}
	return ""
}
