package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type pageController struct{}

var PageController = pageController{}

type listPagesRequest struct {
	Status     string           `json:"status"`
	Pagination PaginationParams `json:"pagination" binding:"required"`
}

var validPageStatusFilters = map[string]bool{
	"": true, "active": true,
	models.PageStatusOpen: true, models.PageStatusAcknowledged: true, models.PageStatusResolved: true,
}

func (c *pageController) List(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var request listPagesRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if !validPageStatusFilters[request.Status] {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status filter"})
		return
	}

	total, err := transactional.PageRepository.CountByProject(tx, projectId, request.Status)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count pages: %w", err))
		return
	}
	offset := (request.Pagination.Page - 1) * request.Pagination.PageSize
	pages, err := transactional.PageRepository.FindByProject(tx, projectId, request.Status, request.Pagination.PageSize, offset)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list pages: %w", err))
		return
	}
	if pages == nil {
		pages = []*models.Page{}
	}

	totalPages := int64(0)
	if request.Pagination.PageSize > 0 {
		totalPages = (int64(total) + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize)
	}
	ctx.JSON(http.StatusOK, PaginatedResponse[*models.Page]{
		Data: pages,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      int64(total),
			TotalPages: totalPages,
		},
	})
}

func (c *pageController) Get(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	page, ok := c.loadPage(ctx)
	if !ok {
		return
	}
	notifications, err := transactional.PageNotificationRepository.FindByPage(tx, page.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load page notifications: %w", err))
		return
	}
	if notifications == nil {
		notifications = []*models.PageNotification{}
	}

	users := map[string]string{}
	members, err := transactional.OrganizationRepository.GetMembersWithDetails(tx, page.OrganizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load members: %w", err))
		return
	}
	for _, member := range members {
		users[strconv.Itoa(member.Id)] = member.Name
	}

	ctx.JSON(http.StatusOK, gin.H{"page": page, "notifications": notifications, "users": users})
}

// Acknowledge deliberately has no write-access gate beyond project read
// access: acking is incident response, and a paged responder must never be
// blocked by a readonly role.
func (c *pageController) Acknowledge(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)
	page, ok := c.loadPage(ctx)
	if !ok {
		return
	}
	acknowledged, err := oncall.AcknowledgePage(tx, page.Id, &userId, oncall.AckViaDashboard, time.Now().UTC())
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to acknowledge page: %w", err))
		return
	}
	if !acknowledged {
		current, err := transactional.PageRepository.FindById(tx, page.Id)
		if err != nil || current == nil {
			ctx.JSON(http.StatusConflict, gin.H{"error": "Page is not open"})
			return
		}
		ctx.JSON(http.StatusConflict, gin.H{"error": "Page is not open", "status": current.Status})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Page acknowledged"})
}

func (c *pageController) Resolve(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)
	page, ok := c.loadPage(ctx)
	if !ok {
		return
	}
	resolved, err := oncall.ResolvePage(tx, page.Id, userId, time.Now().UTC())
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve page: %w", err))
		return
	}
	if !resolved {
		ctx.JSON(http.StatusConflict, gin.H{"error": "Page is already resolved", "status": models.PageStatusResolved})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Page resolved"})
}

func (c *pageController) OpenCount(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	count, err := transactional.PageRepository.CountOpenByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to count open pages: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"count": count})
}

func (c *pageController) loadPage(ctx *gin.Context) (*models.Page, bool) {
	tx := db.GetTx(ctx)
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return nil, false
	}
	pageId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page ID"})
		return nil, false
	}
	page, err := transactional.PageRepository.FindById(tx, pageId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load page: %w", err))
		return nil, false
	}
	if page == nil || page.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return nil, false
	}
	return page, true
}
