package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	traceway "go.tracewayapp.com"
)

type incidentController struct{}

const (
	incidentWindowDays = 90
	incidentListLimit  = 200
	incidentTitleMax   = 200
	incidentMessageMax = 2000
)

func incidentIdFromPath(ctx *gin.Context) (int, bool) {
	id, err := strconv.Atoi(ctx.Param("incidentId"))
	if err != nil || id < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid incident id"})
		return 0, false
	}
	return id, true
}

func invalidateOrgStatusPageCaches(tx *sql.Tx, organizationId int) error {
	pages, err := transactional.StatusPageRepository.ListByOrganization(tx, organizationId)
	if err != nil {
		return err
	}
	for _, page := range pages {
		invalidateStatusPageCache(page.Slug)
	}
	return nil
}

func loadOrgIncident(ctx *gin.Context, tx *sql.Tx, organizationId int) (*models.CheckIncident, bool) {
	incidentId, ok := incidentIdFromPath(ctx)
	if !ok {
		return nil, false
	}
	incident, err := transactional.CheckIncidentRepository.FindByIdInOrganization(tx, incidentId, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load incident: %w", err))
		return nil, false
	}
	if incident == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Incident not found"})
		return nil, false
	}
	return incident, true
}

type incidentView struct {
	*models.OrgIncident
	UpdatesCount int  `json:"updatesCount"`
	PostMortemId *int `json:"postMortemId"`
}

func incidentDecorations(ctx *gin.Context, tx *sql.Tx, incidentIds []int) (map[int]int, map[int]int, bool) {
	counts, err := transactional.IncidentUpdateRepository.CountByIncidentIds(tx, incidentIds)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count incident updates: %w", err))
		return nil, nil, false
	}
	refs, err := transactional.PostMortemRepository.FindRefsByIncidentIds(tx, incidentIds)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load post-mortem refs: %w", err))
		return nil, nil, false
	}
	postMortemIds := map[int]int{}
	for _, ref := range refs {
		if ref.IncidentId != nil {
			postMortemIds[*ref.IncidentId] = ref.Id
		}
	}
	return counts, postMortemIds, true
}

func buildIncidentViews(ctx *gin.Context, tx *sql.Tx, incidents []*models.OrgIncident) ([]incidentView, bool) {
	incidentIds := make([]int, len(incidents))
	for i, incident := range incidents {
		incidentIds[i] = incident.Id
	}
	counts, postMortemIds, ok := incidentDecorations(ctx, tx, incidentIds)
	if !ok {
		return nil, false
	}
	views := make([]incidentView, len(incidents))
	for i, incident := range incidents {
		view := incidentView{OrgIncident: incident, UpdatesCount: counts[incident.Id]}
		if id, ok := postMortemIds[incident.Id]; ok {
			postMortemId := id
			view.PostMortemId = &postMortemId
		}
		views[i] = view
	}
	return views, true
}

func (ctrl *incidentController) List(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	tx := db.GetTx(ctx)
	since := time.Now().UTC().AddDate(0, 0, -incidentWindowDays)
	incidents, err := transactional.CheckIncidentRepository.FindRecentByOrganization(tx, organizationId, since, incidentListLimit)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list incidents: %w", err))
		return
	}
	if incidents == nil {
		incidents = []*models.OrgIncident{}
	}
	views, ok := buildIncidentViews(ctx, tx, incidents)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"incidents": views})
}

func (ctrl *incidentController) ListForStatusPage(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	pageId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || pageId < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status page id"})
		return
	}
	page, pageSize := paginationFromQuery(ctx)

	tx := db.GetTx(ctx)
	statusPage, err := transactional.StatusPageRepository.FindByIdForOrganization(tx, pageId, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load status page: %w", err))
		return
	}
	if statusPage == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Status page not found"})
		return
	}
	var checkIds []int
	if err := json.Unmarshal(statusPage.CheckIds, &checkIds); err != nil {
		checkIds = nil
	}

	incidents, err := transactional.CheckIncidentRepository.FindByStatusPagePaged(tx, statusPage.Id, checkIds, pageSize, (page-1)*pageSize)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list status page incidents: %w", err))
		return
	}
	if incidents == nil {
		incidents = []*models.OrgIncident{}
	}
	total, err := transactional.CheckIncidentRepository.CountByStatusPage(tx, statusPage.Id, checkIds)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count status page incidents: %w", err))
		return
	}
	views, ok := buildIncidentViews(ctx, tx, incidents)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"statusPage": gin.H{"id": statusPage.Id, "name": statusPage.Name},
		"data":       views,
		"pagination": buildPagination(page, pageSize, total),
	})
}

func (ctrl *incidentController) ListUpdates(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	tx := db.GetTx(ctx)
	incident, ok := loadOrgIncident(ctx, tx, organizationId)
	if !ok {
		return
	}
	updates, err := transactional.IncidentUpdateRepository.FindByIncident(tx, incident.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list incident updates: %w", err))
		return
	}
	if updates == nil {
		updates = []*models.IncidentUpdate{}
	}
	ctx.JSON(http.StatusOK, gin.H{"incident": incident, "updates": updates})
}

type manualIncidentRequest struct {
	Title      string `json:"title"`
	StartedAt  string `json:"startedAt"`
	ResolvedAt string `json:"resolvedAt"`
	Message    string `json:"message"`
}

func (ctrl *incidentController) Create(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	pageId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || pageId < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status page id"})
		return
	}
	var req manualIncidentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	tx := db.GetTx(ctx)
	page, err := transactional.StatusPageRepository.FindByIdForOrganization(tx, pageId, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load status page: %w", err))
		return
	}
	if page == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Status page not found"})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Title is required."})
		return
	}
	if utf8.RuneCountInString(req.Title) > incidentTitleMax {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Title must be 200 characters or fewer."})
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if utf8.RuneCountInString(req.Message) > incidentMessageMax {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Message must be 2000 characters or fewer."})
		return
	}
	now := time.Now().UTC()
	startedAt := now
	if req.StartedAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.StartedAt)
		if err != nil {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Started at must be an RFC 3339 timestamp."})
			return
		}
		startedAt = parsed.UTC()
	}
	var resolvedAt *time.Time
	if req.ResolvedAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ResolvedAt)
		if err != nil {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Resolved at must be an RFC 3339 timestamp."})
			return
		}
		utc := parsed.UTC()
		if utc.Before(startedAt) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Resolved at must not be before started at."})
			return
		}
		resolvedAt = &utc
	}

	incident := &models.CheckIncident{
		StatusPageId: &page.Id,
		Title:        req.Title,
		StartedAt:    startedAt,
		ResolvedAt:   resolvedAt,
	}
	incidentId, err := transactional.CheckIncidentRepository.Open(tx, incident)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create incident: %w", err))
		return
	}
	incident.Id = incidentId

	userId := middleware.GetUserId(ctx)
	status := models.IncidentUpdateInvestigating
	updateAt := startedAt
	if resolvedAt != nil {
		status = models.IncidentUpdateResolved
		updateAt = *resolvedAt
	}
	update := &models.IncidentUpdate{
		IncidentId: incidentId,
		Status:     status,
		Message:    req.Message,
		CreatedBy:  &userId,
		CreatedAt:  updateAt,
	}
	if _, err := transactional.IncidentUpdateRepository.Create(tx, update); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create incident update: %w", err))
		return
	}
	if err := invalidateOrgStatusPageCaches(tx, organizationId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to invalidate status page caches: %w", err))
		return
	}
	ctx.JSON(http.StatusCreated, incident)
}

type editIncidentRequest struct {
	Title      string  `json:"title"`
	StartedAt  *string `json:"startedAt"`
	ResolvedAt *string `json:"resolvedAt"`
}

func (ctrl *incidentController) Update(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	var req editIncidentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	tx := db.GetTx(ctx)
	incident, ok := loadOrgIncident(ctx, tx, organizationId)
	if !ok {
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if utf8.RuneCountInString(req.Title) > incidentTitleMax {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Title must be 200 characters or fewer."})
		return
	}
	isManual := incident.StatusPageId != nil
	if isManual && req.Title == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Title is required."})
		return
	}
	if !isManual && (req.StartedAt != nil || req.ResolvedAt != nil) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Start and resolve times of auto-detected incidents are managed by the monitor."})
		return
	}

	if err := transactional.CheckIncidentRepository.UpdateTitle(tx, incident.Id, req.Title); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update incident title: %w", err))
		return
	}
	incident.Title = req.Title

	if isManual && (req.StartedAt != nil || req.ResolvedAt != nil) {
		startedAt := incident.StartedAt
		if req.StartedAt != nil && *req.StartedAt != "" {
			parsed, err := time.Parse(time.RFC3339, *req.StartedAt)
			if err != nil {
				ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Started at must be an RFC 3339 timestamp."})
				return
			}
			startedAt = parsed.UTC()
		}
		resolvedAt := incident.ResolvedAt
		if req.ResolvedAt != nil {
			if *req.ResolvedAt == "" {
				resolvedAt = nil
			} else {
				parsed, err := time.Parse(time.RFC3339, *req.ResolvedAt)
				if err != nil {
					ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Resolved at must be an RFC 3339 timestamp."})
					return
				}
				utc := parsed.UTC()
				resolvedAt = &utc
			}
		}
		if resolvedAt != nil && resolvedAt.Before(startedAt) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Resolved at must not be before started at."})
			return
		}
		if err := transactional.CheckIncidentRepository.UpdateManualTimes(tx, incident.Id, startedAt, resolvedAt); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update incident times: %w", err))
			return
		}
		if incident.ResolvedAt != nil && resolvedAt == nil {
			userId := middleware.GetUserId(ctx)
			reopened := &models.IncidentUpdate{
				IncidentId: incident.Id,
				Status:     models.IncidentUpdateInvestigating,
				Message:    "",
				CreatedBy:  &userId,
				CreatedAt:  time.Now().UTC(),
			}
			if _, err := transactional.IncidentUpdateRepository.Create(tx, reopened); err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create incident update: %w", err))
				return
			}
		}
		incident.StartedAt = startedAt
		incident.ResolvedAt = resolvedAt
	}

	if err := invalidateOrgStatusPageCaches(tx, organizationId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to invalidate status page caches: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, incident)
}

func (ctrl *incidentController) Delete(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	tx := db.GetTx(ctx)
	incident, ok := loadOrgIncident(ctx, tx, organizationId)
	if !ok {
		return
	}
	if incident.StatusPageId == nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Only manually recorded incidents can be deleted."})
		return
	}
	if err := transactional.CheckIncidentRepository.DeleteManual(tx, incident.Id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete incident: %w", err))
		return
	}
	if err := invalidateOrgStatusPageCaches(tx, organizationId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to invalidate status page caches: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

type postIncidentUpdateRequest struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (ctrl *incidentController) CreateUpdate(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	var req postIncidentUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	tx := db.GetTx(ctx)
	incident, ok := loadOrgIncident(ctx, tx, organizationId)
	if !ok {
		return
	}
	if !models.IsValidIncidentUpdateStatus(req.Status) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Status must be one of investigating, identified, monitoring, update, resolved."})
		return
	}
	if req.Status == models.IncidentUpdateResolved && incident.StatusPageId == nil && incident.ResolvedAt == nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Auto-detected incidents resolve when the monitor recovers."})
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if utf8.RuneCountInString(req.Message) > incidentMessageMax {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Message must be 2000 characters or fewer."})
		return
	}

	now := time.Now().UTC()
	userId := middleware.GetUserId(ctx)
	update := &models.IncidentUpdate{
		IncidentId: incident.Id,
		Status:     req.Status,
		Message:    req.Message,
		CreatedBy:  &userId,
		CreatedAt:  now,
	}
	updateId, err := transactional.IncidentUpdateRepository.Create(tx, update)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create incident update: %w", err))
		return
	}
	update.Id = updateId

	if req.Status == models.IncidentUpdateResolved && incident.StatusPageId != nil && incident.ResolvedAt == nil {
		if err := transactional.CheckIncidentRepository.ResolveManual(tx, incident.Id, now); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve incident: %w", err))
			return
		}
	}

	if err := invalidateOrgStatusPageCaches(tx, organizationId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to invalidate status page caches: %w", err))
		return
	}
	ctx.JSON(http.StatusCreated, update)
}

func (ctrl *incidentController) DeleteUpdate(ctx *gin.Context) {
	organizationId, ok := organizationIdFromPath(ctx)
	if !ok {
		return
	}
	updateId, err := strconv.Atoi(ctx.Param("updateId"))
	if err != nil || updateId < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid update id"})
		return
	}
	tx := db.GetTx(ctx)
	incident, ok := loadOrgIncident(ctx, tx, organizationId)
	if !ok {
		return
	}
	update, err := transactional.IncidentUpdateRepository.FindById(tx, updateId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load incident update: %w", err))
		return
	}
	if update == nil || update.IncidentId != incident.Id {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Incident update not found"})
		return
	}
	if err := transactional.IncidentUpdateRepository.Delete(tx, updateId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete incident update: %w", err))
		return
	}
	if err := invalidateOrgStatusPageCaches(tx, organizationId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to invalidate status page caches: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

var IncidentController = incidentController{}
