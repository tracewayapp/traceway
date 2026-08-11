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
	traceway "go.tracewayapp.com"
)

type oncallController struct{}

var OncallController = oncallController{}

type scheduleRequest struct {
	TeamId      int             `json:"teamId"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Timezone    string          `json:"timezone"`
	Definition  models.JSONText `json:"definition"`
}

type createOverrideRequest struct {
	UserId  int       `json:"userId"`
	StartAt time.Time `json:"startAt"`
	EndAt   time.Time `json:"endAt"`
}

type scheduleUserInfo struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (c *oncallController) ListSchedules(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	schedules, err := transactional.OncallScheduleRepository.ListByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list schedules: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"schedules": schedules})
}

func (c *oncallController) CreateSchedule(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	userId := middleware.GetUserId(ctx)

	var request scheduleRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	message, definition, tzName := c.validateScheduleRequest(ctx, organizationId, &request, 0)
	if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	if definition == nil {
		return
	}

	now := time.Now().UTC()
	schedule := &models.OncallSchedule{
		OrganizationId: organizationId,
		TeamId:         request.TeamId,
		Name:           request.Name,
		Description:    request.Description,
		Timezone:       tzName,
		Definition:     definition,
		CreatedBy:      &userId,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	id, err := transactional.OncallScheduleRepository.Create(tx, schedule)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create schedule: %w", err))
		return
	}
	schedule.Id = id
	ctx.JSON(http.StatusCreated, schedule)
}

func (c *oncallController) GetSchedule(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	schedule, ok := c.loadSchedule(ctx, organizationId)
	if !ok {
		return
	}
	now := time.Now().UTC()
	overrides, err := transactional.OncallOverrideRepository.ListForRange(tx, schedule.Id, now, now.AddDate(0, 0, oncall.MaxOverrideDurationDays))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list overrides: %w", err))
		return
	}
	if overrides == nil {
		overrides = []*models.OncallOverride{}
	}
	ctx.JSON(http.StatusOK, gin.H{"schedule": schedule, "overrides": overrides})
}

func (c *oncallController) UpdateSchedule(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	schedule, ok := c.loadSchedule(ctx, organizationId)
	if !ok {
		return
	}
	var request scheduleRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	message, definition, tzName := c.validateScheduleRequest(ctx, organizationId, &request, schedule.Id)
	if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	if definition == nil {
		return
	}

	schedule.TeamId = request.TeamId
	schedule.Name = request.Name
	schedule.Description = request.Description
	schedule.Timezone = tzName
	schedule.Definition = definition
	schedule.UpdatedAt = time.Now().UTC()
	if err := transactional.OncallScheduleRepository.Update(tx, schedule); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update schedule: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, schedule)
}

func (c *oncallController) DeleteSchedule(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	schedule, ok := c.loadSchedule(ctx, organizationId)
	if !ok {
		return
	}

	referencing, err := oncall.PoliciesReferencing(tx, organizationId, oncall.TargetSchedule, schedule.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check referencing policies: %w", err))
		return
	}
	if len(referencing) > 0 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This schedule is used by escalation policy(ies): " + strings.Join(referencing, ", ") + ". Remove those steps first."})
		return
	}

	if err := transactional.OncallScheduleRepository.Delete(tx, schedule.Id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete schedule: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Schedule deleted"})
}

func (c *oncallController) Timeline(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	schedule, ok := c.loadSchedule(ctx, organizationId)
	if !ok {
		return
	}
	from, err := time.Parse(time.RFC3339, ctx.Query("from"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or missing from parameter"})
		return
	}
	to, err := time.Parse(time.RFC3339, ctx.Query("to"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or missing to parameter"})
		return
	}
	from = from.UTC()
	to = to.UTC()
	if !from.Before(to) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "from must be before to"})
		return
	}
	if to.Sub(from) > time.Duration(oncall.MaxTimelineRangeDays)*24*time.Hour {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "The timeline range can span at most 62 days"})
		return
	}

	tz, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		traceway.CaptureException(traceway.NewStackTraceErrorf("schedule %d has an unloadable timezone %q, rendering in UTC: %w", schedule.Id, schedule.Timezone, err))
		tz = time.UTC
	}
	definition, err := oncall.ParseDefinition(schedule.Definition)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("stored schedule definition failed to parse (schedule=%d): %w", schedule.Id, err))
		return
	}
	overrides, err := transactional.OncallOverrideRepository.ListForRange(tx, schedule.Id, from, to)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list overrides: %w", err))
		return
	}

	type layerTimeline struct {
		Id     string         `json:"id"`
		Name   string         `json:"name"`
		Shifts []oncall.Shift `json:"shifts"`
	}
	layers := make([]layerTimeline, 0, len(definition.Layers))
	userIds := map[int]bool{}
	for i := range definition.Layers {
		layer := &definition.Layers[i]
		shifts := oncall.ResolveLayerRange(layer, tz, from, to)
		if shifts == nil {
			shifts = []oncall.Shift{}
		}
		for _, shift := range shifts {
			userIds[shift.UserId] = true
		}
		layers = append(layers, layerTimeline{Id: layer.Id, Name: layer.Name, Shifts: shifts})
	}
	final := oncall.ResolveRange(definition, tz, overrides, from, to)
	if final == nil {
		final = []oncall.Shift{}
	}
	for _, shift := range final {
		userIds[shift.UserId] = true
	}

	members, err := transactional.OrganizationRepository.GetMembersWithDetails(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load members: %w", err))
		return
	}
	memberById := map[int]*models.OrganizationMember{}
	for _, member := range members {
		memberById[member.Id] = member
	}
	users := map[string]*scheduleUserInfo{}
	for userId := range userIds {
		if member, ok := memberById[userId]; ok {
			users[strconv.Itoa(userId)] = &scheduleUserInfo{Name: member.Name, Email: member.Email}
		} else {
			users[strconv.Itoa(userId)] = nil
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"schedule": gin.H{"id": schedule.Id, "name": schedule.Name, "timezone": schedule.Timezone, "teamId": schedule.TeamId},
		"from":     from,
		"to":       to,
		"layers":   layers,
		"final":    final,
		"users":    users,
	})
}

func (c *oncallController) CreateOverride(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	userId := middleware.GetUserId(ctx)

	schedule, ok := c.loadSchedule(ctx, organizationId)
	if !ok {
		return
	}
	var request createOverrideRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if !request.StartAt.Before(request.EndAt) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The override must end after it starts."})
		return
	}
	if request.EndAt.Sub(request.StartAt) > time.Duration(oncall.MaxOverrideDurationDays)*24*time.Hour {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "An override can last at most 30 days."})
		return
	}
	role, err := transactional.OrganizationRepository.GetUserRole(tx, organizationId, request.UserId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check override user: %w", err))
		return
	}
	if role == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The covering user must be a member of the organization."})
		return
	}

	override := &models.OncallOverride{
		ScheduleId: schedule.Id,
		UserId:     request.UserId,
		StartAt:    request.StartAt.UTC(),
		EndAt:      request.EndAt.UTC(),
		CreatedBy:  &userId,
		CreatedAt:  time.Now().UTC(),
	}
	id, err := transactional.OncallOverrideRepository.Create(tx, override)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create override: %w", err))
		return
	}
	override.Id = id
	ctx.JSON(http.StatusCreated, override)
}

func (c *oncallController) DeleteOverride(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)
	userId := middleware.GetUserId(ctx)
	role := middleware.GetUserOrgRole(ctx)

	schedule, ok := c.loadSchedule(ctx, organizationId)
	if !ok {
		return
	}
	overrideId, err := strconv.Atoi(ctx.Param("overrideId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid override ID"})
		return
	}
	override, err := transactional.OncallOverrideRepository.FindById(tx, overrideId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load override: %w", err))
		return
	}
	if override == nil || override.ScheduleId != schedule.Id {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Override not found"})
		return
	}

	isAdmin := role == "owner" || role == "admin"
	isCreator := override.CreatedBy != nil && *override.CreatedBy == userId
	isCovering := override.UserId == userId
	if !isAdmin && !isCreator && !isCovering {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only the creator, the covering user, or an admin can delete an override"})
		return
	}
	if err := transactional.OncallOverrideRepository.Delete(tx, override.Id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete override: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Override deleted"})
}

// Now is the org-wide overview: per team, per schedule, who is on call and who
// is next.
func (c *oncallController) Now(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	organizationId := middleware.GetOrganizationId(ctx)

	teams, err := transactional.TeamRepository.ListByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list teams: %w", err))
		return
	}
	schedules, err := transactional.OncallScheduleRepository.ListByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list schedules: %w", err))
		return
	}
	members, err := transactional.OrganizationRepository.GetMembersWithDetails(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load members: %w", err))
		return
	}
	memberById := map[int]*models.OrganizationMember{}
	for _, member := range members {
		memberById[member.Id] = member
	}

	now := time.Now().UTC()
	allOverrides, err := transactional.OncallOverrideRepository.ListForRangeByOrganization(tx, organizationId, now, now.AddDate(0, 0, 35))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list overrides: %w", err))
		return
	}
	overridesBySchedule := map[int][]*models.OncallOverride{}
	for _, override := range allOverrides {
		overridesBySchedule[override.ScheduleId] = append(overridesBySchedule[override.ScheduleId], override)
	}

	type scheduleNow struct {
		Id     int                 `json:"id"`
		Name   string              `json:"name"`
		Oncall []oncall.OncallUser `json:"oncall"`
		Until  *time.Time          `json:"until"`
		NextUp *oncall.OncallUser  `json:"nextUp"`
		NextAt *time.Time          `json:"nextAt"`
	}
	type teamNow struct {
		Team      *models.TeamWithCounts `json:"team"`
		Schedules []scheduleNow          `json:"schedules"`
	}

	schedulesByTeam := map[int][]*models.OncallSchedule{}
	for _, schedule := range schedules {
		schedulesByTeam[schedule.TeamId] = append(schedulesByTeam[schedule.TeamId], schedule)
	}

	response := make([]teamNow, 0, len(teams))
	for _, team := range teams {
		entry := teamNow{Team: team, Schedules: []scheduleNow{}}
		for _, schedule := range schedulesByTeam[team.Id] {
			tz, err := time.LoadLocation(schedule.Timezone)
			if err != nil {
				traceway.CaptureException(traceway.NewStackTraceErrorf("schedule %d has an unloadable timezone %q, rendering in UTC: %w", schedule.Id, schedule.Timezone, err))
				tz = time.UTC
			}
			definition, err := oncall.ParseDefinition(schedule.Definition)
			if err != nil {
				traceway.CaptureException(traceway.NewStackTraceErrorf("stored schedule definition failed to parse (schedule=%d): %w", schedule.Id, err))
				continue
			}
			overrides := overridesBySchedule[schedule.Id]
			item := scheduleNow{Id: schedule.Id, Name: schedule.Name, Oncall: []oncall.OncallUser{}}
			for _, onCallUserId := range oncall.ResolveAt(definition, tz, overrides, now) {
				if member, ok := memberById[onCallUserId]; ok {
					item.Oncall = append(item.Oncall, oncall.OncallUser{UserId: onCallUserId, Name: member.Name, Email: member.Email})
				}
			}
			current, next := oncall.CurrentAndNext(definition, tz, overrides, now)
			if current != nil {
				until := current.End
				item.Until = &until
			}
			if next != nil {
				if member, ok := memberById[next.UserId]; ok {
					item.NextUp = &oncall.OncallUser{UserId: next.UserId, Name: member.Name, Email: member.Email}
					nextAt := next.Start
					item.NextAt = &nextAt
				}
			}
			entry.Schedules = append(entry.Schedules, item)
		}
		response = append(response, entry)
	}
	ctx.JSON(http.StatusOK, gin.H{"teams": response})
}

// Current is the project-scoped ownership seam: the owning team and current
// on-call for the project, consumed by the issue page and the escalation
// engine's UI.
func (c *oncallController) Current(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	result, err := oncall.CurrentOnCallForProject(tx, projectId, time.Now().UTC())
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve current on-call: %w", err))
		return
	}
	if result == nil {
		ctx.JSON(http.StatusOK, gin.H{"team": nil, "schedules": []oncall.ScheduleRef{}, "oncall": []oncall.OncallUser{}})
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *oncallController) loadSchedule(ctx *gin.Context, organizationId int) (*models.OncallSchedule, bool) {
	tx := db.GetTx(ctx)
	scheduleId, err := strconv.Atoi(ctx.Param("scheduleId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule ID"})
		return nil, false
	}
	schedule, err := transactional.OncallScheduleRepository.FindById(tx, scheduleId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load schedule: %w", err))
		return nil, false
	}
	if schedule == nil || schedule.OrganizationId != organizationId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Schedule not found"})
		return nil, false
	}
	return schedule, true
}

// validateScheduleRequest validates the shared create/update payload. It
// returns a 422 message, the normalized definition, and the timezone to store.
// A nil definition together with an empty message means a 500 was already
// written.
func (c *oncallController) validateScheduleRequest(ctx *gin.Context, organizationId int, request *scheduleRequest, currentScheduleId int) (string, models.JSONText, string) {
	tx := db.GetTx(ctx)

	if request.Name == "" {
		return "A schedule name is required.", nil, ""
	}
	if len(request.Name) > 100 {
		return "The schedule name can be at most 100 characters.", nil, ""
	}
	team, err := transactional.TeamRepository.FindById(tx, request.TeamId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load team: %w", err))
		return "", nil, ""
	}
	if team == nil || team.OrganizationId != organizationId {
		return "The schedule must belong to a team in this organization.", nil, ""
	}
	existing, err := transactional.OncallScheduleRepository.FindByOrganizationAndName(tx, organizationId, request.Name)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check schedule name: %w", err))
		return "", nil, ""
	}
	if existing != nil && existing.Id != currentScheduleId {
		return "A schedule with this name already exists.", nil, ""
	}

	tzName := request.Timezone
	if tzName == "" {
		organization, err := transactional.OrganizationRepository.FindById(tx, organizationId)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load organization: %w", err))
			return "", nil, ""
		}
		if organization != nil && organization.Timezone != "" {
			tzName = organization.Timezone
		} else {
			tzName = "UTC"
		}
	}
	if _, err := oncall.LoadTimezone(tzName); err != nil {
		return err.Error(), nil, ""
	}

	definition, err := oncall.ParseDefinition(request.Definition)
	if err != nil {
		return err.Error(), nil, ""
	}
	memberMessage, err := c.checkLayerMembers(tx, organizationId, definition)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to validate schedule members: %w", err))
		return "", nil, ""
	}
	if memberMessage != "" {
		return memberMessage, nil, ""
	}
	normalized, err := oncall.MarshalDefinition(definition)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to marshal schedule definition: %w", err))
		return "", nil, ""
	}
	return "", models.JSONText(normalized), tzName
}

func (c *oncallController) checkLayerMembers(tx *sql.Tx, organizationId int, definition *models.OncallScheduleDefinition) (string, error) {
	members, err := transactional.OrganizationRepository.GetMembersWithDetails(tx, organizationId)
	if err != nil {
		return "", err
	}
	memberSet := make(map[int]bool, len(members))
	for _, member := range members {
		memberSet[member.Id] = true
	}
	for _, layer := range definition.Layers {
		for _, layerUserId := range layer.UserIds {
			if !memberSet[layerUserId] {
				return "Layer \"" + layer.Name + "\" includes someone who is not a member of the organization.", nil
			}
		}
	}
	return "", nil
}
