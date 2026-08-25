package controllers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
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

// pageResponse decorates a page with the exception hash it was opened for
// (empty for pages not linked to an issue), so the dashboard can offer
// archiving the issue when the page is resolved, plus the display names of
// the acknowledging/resolving users for the list columns.
type pageResponse struct {
	*models.Page
	IssueHash          string `json:"issueHash"`
	AcknowledgedByName string `json:"acknowledgedByName,omitempty"`
	ResolvedByName     string `json:"resolvedByName,omitempty"`
}

func toPageResponse(page *models.Page, names map[int]string) pageResponse {
	response := pageResponse{Page: page, IssueHash: page.IssueHash()}
	if page.AcknowledgedBy != nil {
		response.AcknowledgedByName = names[*page.AcknowledgedBy]
	}
	if page.ResolvedBy != nil {
		response.ResolvedByName = names[*page.ResolvedBy]
	}
	return response
}

// memberNames maps the organization's member ids to display names; users no
// longer in the organization simply stay absent (the frontend falls back to
// "user #id").
func memberNames(tx *sql.Tx, organizationId int) (map[int]string, error) {
	members, err := transactional.OrganizationRepository.GetMembersWithDetails(tx, organizationId)
	if err != nil {
		return nil, err
	}
	names := make(map[int]string, len(members))
	for _, member := range members {
		names[member.Id] = member.Name
	}
	return names, nil
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
	names := map[int]string{}
	for _, page := range pages {
		if page.AcknowledgedBy != nil || page.ResolvedBy != nil {
			names, err = memberNames(tx, page.OrganizationId)
			if err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load members: %w", err))
				return
			}
			break
		}
	}
	items := make([]pageResponse, 0, len(pages))
	for _, page := range pages {
		items = append(items, toPageResponse(page, names))
	}

	totalPages := int64(0)
	if request.Pagination.PageSize > 0 {
		totalPages = (int64(total) + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize)
	}
	ctx.JSON(http.StatusOK, PaginatedResponse[pageResponse]{
		Data: items,
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

	names, err := memberNames(tx, page.OrganizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load members: %w", err))
		return
	}
	users := make(map[string]string, len(names))
	for id, name := range names {
		users[strconv.Itoa(id)] = name
	}

	ctx.JSON(http.StatusOK, gin.H{"page": toPageResponse(page, names), "notifications": notifications, "users": users})
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

type resolvePageRequest struct {
	ArchiveIssue bool `json:"archiveIssue"`
}

func (c *pageController) Resolve(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)
	page, ok := c.loadPage(ctx)
	if !ok {
		return
	}
	// The body is optional: the endpoint historically took none, so a missing
	// or empty body just means no archive.
	var request resolvePageRequest
	_ = ctx.ShouldBindJSON(&request)

	resolved, err := oncall.ResolvePage(tx, page.Id, userId, time.Now().UTC())
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve page: %w", err))
		return
	}
	if !resolved {
		ctx.JSON(http.StatusConflict, gin.H{"error": "Page is already resolved", "status": models.PageStatusResolved})
		return
	}

	archived := false
	if request.ArchiveIssue {
		hash := page.IssueHash()
		if hash == "" {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This page is not linked to an issue"})
			return
		}
		// Resolving deliberately has no write gate, but archiving an issue is
		// a write: enforce the effective project role in-handler. Aborting
		// rolls the resolve back with it, keeping the action all-or-nothing.
		role, err := transactional.ProjectRepository.GetEffectiveRole(tx, page.ProjectId, userId)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve effective role: %w", err))
			return
		}
		if role == "readonly" || role == "" {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Read-only access. Archiving the issue is not permitted."})
			return
		}
		if err := telemetry.ExceptionStackTraceRepository.ArchiveByHashes(ctx, page.ProjectId, []string{hash}); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to archive issue %s for page %d: %w", hash, page.Id, err))
			return
		}
		archived = true
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Page resolved", "archivedIssue": archived})
}

type bulkAcknowledgeRequest struct {
	Ids []int `json:"ids"`
}

const maxBulkPageIds = 100

// BulkAcknowledge acknowledges every selected page that is still open. Pages
// from other projects or in another state are skipped, not errors: bulk
// actions race the escalator and other responders by nature.
func (c *pageController) BulkAcknowledge(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var request bulkAcknowledgeRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || len(request.Ids) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "ids array is required"})
		return
	}
	if len(request.Ids) > maxBulkPageIds {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("a maximum of %d pages can be acknowledged at once", maxBulkPageIds)})
		return
	}

	now := time.Now().UTC()
	acknowledged := 0
	for _, id := range request.Ids {
		page, err := transactional.PageRepository.FindById(tx, id)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load page %d: %w", id, err))
			return
		}
		if page == nil || page.ProjectId != projectId {
			continue
		}
		ok, err := oncall.AcknowledgePage(tx, page.Id, &userId, oncall.AckViaDashboard, now)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to acknowledge page %d: %w", id, err))
			return
		}
		if ok {
			acknowledged++
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"acknowledged": acknowledged})
}

type bulkResolveRequest struct {
	Ids           []int `json:"ids"`
	ArchiveIssues bool  `json:"archiveIssues"`
}

// BulkResolve resolves every selected page that is not already resolved,
// optionally archiving the distinct issues the selected pages were opened
// for. Issue hashes are collected from the whole valid selection, not just
// the pages this call transitioned: a page someone else resolved a moment
// earlier was still selected with the intent to archive its issue.
func (c *pageController) BulkResolve(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var request bulkResolveRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || len(request.Ids) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "ids array is required"})
		return
	}
	if len(request.Ids) > maxBulkPageIds {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("a maximum of %d pages can be resolved at once", maxBulkPageIds)})
		return
	}

	now := time.Now().UTC()
	resolved := 0
	hashSet := map[string]bool{}
	for _, id := range request.Ids {
		page, err := transactional.PageRepository.FindById(tx, id)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load page %d: %w", id, err))
			return
		}
		if page == nil || page.ProjectId != projectId {
			continue
		}
		ok, err := oncall.ResolvePage(tx, page.Id, userId, now)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve page %d: %w", id, err))
			return
		}
		if ok {
			resolved++
		}
		if request.ArchiveIssues {
			if hash := page.IssueHash(); hash != "" {
				hashSet[hash] = true
			}
		}
	}

	archivedIssues := 0
	if len(hashSet) > 0 {
		// Resolving deliberately has no write gate, but archiving issues is a
		// write: enforce the effective project role in-handler. Aborting rolls
		// the resolves back with it, keeping the action all-or-nothing.
		role, err := transactional.ProjectRepository.GetEffectiveRole(tx, projectId, userId)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve effective role: %w", err))
			return
		}
		if role == "readonly" || role == "" {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Read-only access. Archiving the issues is not permitted."})
			return
		}
		hashes := make([]string, 0, len(hashSet))
		for hash := range hashSet {
			hashes = append(hashes, hash)
		}
		if err := telemetry.ExceptionStackTraceRepository.ArchiveByHashes(ctx, projectId, hashes); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to archive issues for resolved pages: %w", err))
			return
		}
		archivedIssues = len(hashes)
	}
	ctx.JSON(http.StatusOK, gin.H{"resolved": resolved, "archivedIssues": archivedIssues})
}

type pagesForIssuesRequest struct {
	Hashes []string `json:"hashes" binding:"required"`
}

// UnresolvedForIssues reports how many unresolved pages were opened for the
// given exception hashes, so the archive dialog can offer resolving them.
func (c *pageController) UnresolvedForIssues(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var request pagesForIssuesRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if len(request.Hashes) > maxBulkIssueHashes {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("a maximum of %d issues can be checked at once", maxBulkIssueHashes)})
		return
	}

	count := 0
	for _, hash := range request.Hashes {
		pages, err := transactional.PageRepository.FindUnresolvedByIssueHash(tx, projectId, hash)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find pages for issue %s: %w", hash, err))
			return
		}
		count += len(pages)
	}
	ctx.JSON(http.StatusOK, gin.H{"count": count})
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
