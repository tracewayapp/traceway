package controllers

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	traceway "go.tracewayapp.com"
)

type postMortemController struct{}

const (
	postMortemTitleMax   = 200
	postMortemContentMax = 200_000
	postMortemTagMax     = 50
	postMortemMaxTags    = 20
)

const postMortemDuplicateMessage = "This incident already has a post-mortem."

type postMortemRequest struct {
	Title      string   `json:"title"`
	ContentMd  string   `json:"contentMd"`
	Tags       []string `json:"tags"`
	IncidentId *int     `json:"incidentId"`
}

func normalizePostMortemTags(tags []string) (models.StringSlice, string) {
	normalized := models.StringSlice{}
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		tag = strings.NewReplacer("\"", "", "\\", "", ",", "").Replace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		if utf8.RuneCountInString(tag) > postMortemTagMax {
			return nil, "Tags must be 50 characters or fewer."
		}
		seen[tag] = true
		normalized = append(normalized, tag)
	}
	if len(normalized) > postMortemMaxTags {
		return nil, "A post-mortem can have at most 20 tags."
	}
	return normalized, ""
}

func validatePostMortemRequest(req *postMortemRequest) (models.StringSlice, string) {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return nil, "Title is required."
	}
	if utf8.RuneCountInString(req.Title) > postMortemTitleMax {
		return nil, "Title must be 200 characters or fewer."
	}
	if len(req.ContentMd) > postMortemContentMax {
		return nil, "Content must be 200KB or smaller."
	}
	return normalizePostMortemTags(req.Tags)
}

func intPtrEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// postMortemProjectContext resolves the project from RequireProjectAccess and
// its owning organization; post-mortems belong to a project, but incident
// links are still validated against the organization.
func postMortemProjectContext(ctx *gin.Context) (uuid.UUID, int, bool) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return uuid.Nil, 0, false
	}
	tx := db.GetTx(ctx)
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load project: %w", err))
		return uuid.Nil, 0, false
	}
	if project == nil || project.OrganizationId == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return uuid.Nil, 0, false
	}
	return projectId, *project.OrganizationId, true
}

func checkIncidentLink(ctx *gin.Context, organizationId int, projectId uuid.UUID, incidentId int) bool {
	tx := db.GetTx(ctx)
	incident, err := transactional.CheckIncidentRepository.FindByIdInOrganization(tx, incidentId, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load incident: %w", err))
		return false
	}
	if incident == nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The linked incident does not belong to this organization."})
		return false
	}
	if incident.ProjectId != nil && *incident.ProjectId != projectId {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The linked incident belongs to a different project."})
		return false
	}
	return true
}

func (ctrl *postMortemController) List(ctx *gin.Context) {
	projectId, _, ok := postMortemProjectContext(ctx)
	if !ok {
		return
	}
	search := strings.TrimSpace(ctx.Query("search"))
	tags := []string{}
	for _, tag := range ctx.QueryArray("tag") {
		if tag = strings.ToLower(strings.TrimSpace(tag)); tag != "" {
			tags = append(tags, tag)
		}
	}
	page, pageSize := paginationFromQuery(ctx)

	tx := db.GetTx(ctx)
	items, err := transactional.PostMortemRepository.ListByProject(tx, projectId, search, tags, pageSize, (page-1)*pageSize)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list post-mortems: %w", err))
		return
	}
	if items == nil {
		items = []*models.PostMortemListItem{}
	}
	for _, item := range items {
		if item.Tags == nil {
			item.Tags = models.StringSlice{}
		}
	}
	total, err := transactional.PostMortemRepository.CountByProject(tx, projectId, search, tags)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count post-mortems: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"data":       items,
		"pagination": buildPagination(page, pageSize, total),
	})
}

func (ctrl *postMortemController) Get(ctx *gin.Context) {
	projectId, _, ok := postMortemProjectContext(ctx)
	if !ok {
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post-mortem id"})
		return
	}
	tx := db.GetTx(ctx)
	postMortem, err := transactional.PostMortemRepository.FindDetailByIdForProject(tx, id, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load post-mortem: %w", err))
		return
	}
	if postMortem == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Post-mortem not found"})
		return
	}
	if postMortem.Tags == nil {
		postMortem.Tags = models.StringSlice{}
	}
	ctx.JSON(http.StatusOK, postMortem)
}

func (ctrl *postMortemController) Activity(ctx *gin.Context) {
	projectId, _, ok := postMortemProjectContext(ctx)
	if !ok {
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post-mortem id"})
		return
	}
	tx := db.GetTx(ctx)
	events, err := transactional.PostMortemRepository.ListEvents(tx, id, projectId, 200)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list post-mortem activity: %w", err))
		return
	}
	if events == nil {
		events = []*models.PostMortemEventItem{}
	}
	for _, event := range events {
		if event.Changes == nil {
			event.Changes = models.StringSlice{}
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"events": events})
}

func (ctrl *postMortemController) Create(ctx *gin.Context) {
	projectId, organizationId, ok := postMortemProjectContext(ctx)
	if !ok {
		return
	}
	var req postMortemRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	tx := db.GetTx(ctx)
	tags, message := validatePostMortemRequest(&req)
	if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	if req.IncidentId != nil && !checkIncidentLink(ctx, organizationId, projectId, *req.IncidentId) {
		return
	}

	userId := middleware.GetUserId(ctx)
	now := time.Now().UTC()
	postMortem := &models.PostMortem{
		OrganizationId: organizationId,
		ProjectId:      projectId,
		IncidentId:     req.IncidentId,
		Title:          req.Title,
		ContentMd:      req.ContentMd,
		Tags:           tags,
		CreatedBy:      &userId,
		UpdatedBy:      &userId,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	id, err := transactional.PostMortemRepository.Create(tx, postMortem)
	if err != nil {
		if db.IsUniqueViolation(err) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": postMortemDuplicateMessage})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create post-mortem: %w", err))
		return
	}
	postMortem.Id = id
	if err := transactional.PostMortemRepository.RecordEvent(tx, &models.PostMortemEvent{
		PostMortemId: id,
		UserId:       &userId,
		Action:       models.PostMortemEventCreated,
		Changes:      models.StringSlice{},
		CreatedAt:    now,
	}); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to record post-mortem event: %w", err))
		return
	}
	ctx.JSON(http.StatusCreated, postMortem)
}

func (ctrl *postMortemController) Update(ctx *gin.Context) {
	projectId, organizationId, ok := postMortemProjectContext(ctx)
	if !ok {
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post-mortem id"})
		return
	}
	var req postMortemRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	tx := db.GetTx(ctx)
	postMortem, err := transactional.PostMortemRepository.FindByIdForProject(tx, id, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load post-mortem: %w", err))
		return
	}
	if postMortem == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Post-mortem not found"})
		return
	}
	tags, message := validatePostMortemRequest(&req)
	if message != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": message})
		return
	}
	if req.IncidentId != nil && !checkIncidentLink(ctx, organizationId, projectId, *req.IncidentId) {
		return
	}

	changes := models.StringSlice{}
	if postMortem.Title != req.Title {
		changes = append(changes, "title")
	}
	if postMortem.ContentMd != req.ContentMd {
		changes = append(changes, "content")
	}
	if !slices.Equal(postMortem.Tags, tags) {
		changes = append(changes, "tags")
	}
	if !intPtrEqual(postMortem.IncidentId, req.IncidentId) {
		changes = append(changes, "incident link")
	}
	if len(changes) == 0 {
		ctrl.Get(ctx)
		return
	}

	userId := middleware.GetUserId(ctx)
	now := time.Now().UTC()
	postMortem.IncidentId = req.IncidentId
	postMortem.Title = req.Title
	postMortem.ContentMd = req.ContentMd
	postMortem.Tags = tags
	postMortem.UpdatedBy = &userId
	postMortem.UpdatedAt = now
	if err := transactional.PostMortemRepository.Update(tx, postMortem); err != nil {
		if db.IsUniqueViolation(err) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": postMortemDuplicateMessage})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update post-mortem: %w", err))
		return
	}
	if err := transactional.PostMortemRepository.RecordEvent(tx, &models.PostMortemEvent{
		PostMortemId: id,
		UserId:       &userId,
		Action:       models.PostMortemEventUpdated,
		Changes:      changes,
		CreatedAt:    now,
	}); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to record post-mortem event: %w", err))
		return
	}
	detail, err := transactional.PostMortemRepository.FindDetailByIdForProject(tx, id, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reload post-mortem after update: %w", err))
		return
	}
	if detail == nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("post-mortem %d vanished during update", id))
		return
	}
	if detail.Tags == nil {
		detail.Tags = models.StringSlice{}
	}
	ctx.JSON(http.StatusOK, detail)
}

func (ctrl *postMortemController) Delete(ctx *gin.Context) {
	projectId, _, ok := postMortemProjectContext(ctx)
	if !ok {
		return
	}
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id < 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post-mortem id"})
		return
	}
	tx := db.GetTx(ctx)
	postMortem, err := transactional.PostMortemRepository.FindByIdForProject(tx, id, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load post-mortem: %w", err))
		return
	}
	if postMortem == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Post-mortem not found"})
		return
	}
	if err := transactional.PostMortemRepository.Delete(tx, id, projectId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete post-mortem: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

var PostMortemController = postMortemController{}
