package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type widgetController struct{}

type AddWidgetRequest struct {
	Title      string          `json:"title"`
	WidgetType string          `json:"widgetType" binding:"required"`
	Config     json.RawMessage `json:"config"`
}

func (c *widgetController) Add(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	groupId, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget group id"})
		return
	}

	var req AddWidgetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.Config == nil {
		req.Config = json.RawMessage(`{}`)
	}
	if msg := validateWidgetConfig(req.Config); msg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Title is required."})
		return
	}

	tx := db.GetTx(ctx)

	group, err := transactional.WidgetGroupRepository.FindById(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to add widget: %w", err))
		return
	}
	if group == nil || group.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget group not found"})
		return
	}

	existing, err := transactional.WidgetGroupRepository.FindWidgetsByGroup(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to add widget: %w", err))
		return
	}

	w := &models.WidgetGroupWidget{
		WidgetGroupId: groupId,
		Title:         req.Title,
		WidgetType:    req.WidgetType,
		Config:        req.Config,
		Position:      len(existing),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	id, err := transactional.WidgetGroupRepository.CreateWidget(tx, w)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to add widget: %w", err))
		return
	}
	w.Id = id

	ctx.JSON(http.StatusCreated, w)
}

func (c *widgetController) Update(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	groupIdStr := ctx.Param("id")
	groupId, err := strconv.Atoi(groupIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget group id"})
		return
	}

	widgetIdStr := ctx.Param("wid")
	widgetId, err := strconv.Atoi(widgetIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget id"})
		return
	}

	var req AddWidgetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.Config != nil {
		if msg := validateWidgetConfig(req.Config); msg != "" {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
			return
		}
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Title is required."})
		return
	}

	tx := db.GetTx(ctx)

	group, err := transactional.WidgetGroupRepository.FindById(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update widget: %w", err))
		return
	}
	if group == nil || group.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget group not found"})
		return
	}

	widget, err := transactional.WidgetGroupRepository.FindWidgetById(tx, widgetId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update widget: %w", err))
		return
	}
	if widget == nil || widget.WidgetGroupId != groupId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget not found"})
		return
	}

	widget.Title = req.Title
	widget.WidgetType = req.WidgetType
	if req.Config != nil {
		widget.Config = req.Config
	}
	widget.UpdatedAt = time.Now().UTC()

	if err := transactional.WidgetGroupRepository.UpdateWidget(tx, widget); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update widget: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, widget)
}

type ReorderWidgetsRequest struct {
	WidgetIds []int `json:"widgetIds" binding:"required,min=1"`
}

func (c *widgetController) Reorder(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	groupIdStr := ctx.Param("id")
	groupId, err := strconv.Atoi(groupIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget group id"})
		return
	}

	var req ReorderWidgetsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)

	group, err := transactional.WidgetGroupRepository.FindById(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reorder widgets: %w", err))
		return
	}
	if group == nil || group.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget group not found"})
		return
	}

	allWidgets, err := transactional.WidgetGroupRepository.FindWidgetsByGroup(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reorder widgets: %w", err))
		return
	}

	byId := make(map[int]*models.WidgetGroupWidget, len(allWidgets))
	for _, w := range allWidgets {
		byId[w.Id] = w
	}

	if len(req.WidgetIds) != len(allWidgets) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The widget list is out of date. Please refresh and try again."})
		return
	}

	seen := make(map[int]bool, len(req.WidgetIds))
	for _, id := range req.WidgetIds {
		if byId[id] == nil || seen[id] {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The widget list is out of date. Please refresh and try again."})
			return
		}
		seen[id] = true
	}

	now := time.Now().UTC()
	for position, id := range req.WidgetIds {
		w := byId[id]
		if w.Position == position {
			continue
		}
		w.Position = position
		w.UpdatedAt = now
		if err := transactional.WidgetGroupRepository.UpdateWidget(tx, w); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reorder widgets: %w", err))
			return
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"reordered": true})
}

func (c *widgetController) Delete(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	groupIdStr := ctx.Param("id")
	groupId, err := strconv.Atoi(groupIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget group id"})
		return
	}

	widgetIdStr := ctx.Param("wid")
	widgetId, err := strconv.Atoi(widgetIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget id"})
		return
	}

	tx := db.GetTx(ctx)

	group, err := transactional.WidgetGroupRepository.FindById(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete widget: %w", err))
		return
	}
	if group == nil || group.ProjectId != projectId {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}

	widget, err := transactional.WidgetGroupRepository.FindWidgetById(tx, widgetId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete widget: %w", err))
		return
	}
	if widget == nil || widget.WidgetGroupId != groupId {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}

	if err := transactional.WidgetGroupRepository.DeleteStarredByWidgetId(tx, widgetId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete widget: %w", err))
		return
	}

	if err := transactional.WidgetGroupRepository.DeleteWidget(tx, widgetId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete widget: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (c *widgetController) ToggleStar(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	groupIdStr := ctx.Param("id")
	groupId, err := strconv.Atoi(groupIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget group id"})
		return
	}

	widgetIdStr := ctx.Param("wid")
	widgetId, err := strconv.Atoi(widgetIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget id"})
		return
	}

	tx := db.GetTx(ctx)

	group, err := transactional.WidgetGroupRepository.FindById(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
		return
	}
	if group == nil || group.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget group not found"})
		return
	}

	widget, err := transactional.WidgetGroupRepository.FindWidgetById(tx, widgetId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
		return
	}
	if widget == nil || widget.WidgetGroupId != groupId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget not found"})
		return
	}

	starred, err := transactional.WidgetGroupRepository.FindStarredByWidgetId(tx, projectId, widgetId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
		return
	}

	if starred != nil {
		if err := transactional.WidgetGroupRepository.DeleteStarredByWidgetId(tx, widgetId); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"id": widgetId, "isStarred": false})
		return
	}

	allStarred, err := transactional.WidgetGroupRepository.FindStarredByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
		return
	}

	var cfg struct {
		ColSpan int    `json:"colSpan"`
		Size    string `json:"size"`
	}
	_ = json.Unmarshal(widget.Config, &cfg)
	if cfg.ColSpan < 1 || cfg.ColSpan > 3 {
		cfg.ColSpan = 1
	}
	if cfg.Size != "sm" && cfg.Size != "md" && cfg.Size != "lg" {
		cfg.Size = "sm"
	}

	nextPosition := 0
	for _, s := range allStarred {
		if s.Position >= nextPosition {
			nextPosition = s.Position + 1
		}
	}

	row := &models.StarredWidget{
		WidgetId:  widgetId,
		Position:  nextPosition,
		ColSpan:   cfg.ColSpan,
		Size:      cfg.Size,
		CreatedAt: time.Now().UTC(),
	}
	if err := transactional.WidgetGroupRepository.CreateStarred(tx, row); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"id": widgetId, "isStarred": true})
}

func (c *widgetController) ListStarred(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)

	widgets, err := transactional.WidgetGroupRepository.FindStarredWidgetsByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list starred widgets: %w", err))
		return
	}

	if widgets == nil {
		widgets = []*models.StarredWidgetWithHome{}
	}

	ctx.JSON(http.StatusOK, widgets)
}

func (c *widgetController) ReorderStarred(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var req ReorderWidgetsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)

	starred, err := transactional.WidgetGroupRepository.FindStarredByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reorder starred widgets: %w", err))
		return
	}

	byWidgetId := make(map[int]*models.StarredWidget, len(starred))
	for _, s := range starred {
		byWidgetId[s.WidgetId] = s
	}

	if len(req.WidgetIds) != len(starred) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The widget list is out of date. Please refresh and try again."})
		return
	}

	seen := make(map[int]bool, len(req.WidgetIds))
	for _, id := range req.WidgetIds {
		if byWidgetId[id] == nil || seen[id] {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The widget list is out of date. Please refresh and try again."})
			return
		}
		seen[id] = true
	}

	for position, id := range req.WidgetIds {
		s := byWidgetId[id]
		if s.Position == position {
			continue
		}
		s.Position = position
		if err := transactional.WidgetGroupRepository.UpdateStarred(tx, s); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reorder starred widgets: %w", err))
			return
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"reordered": true})
}

type UpdateStarredLayoutRequest struct {
	ColSpan int    `json:"colSpan"`
	Size    string `json:"size"`
}

func (c *widgetController) UpdateStarredLayout(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	widgetIdStr := ctx.Param("wid")
	widgetId, err := strconv.Atoi(widgetIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget id"})
		return
	}

	var req UpdateStarredLayoutRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.ColSpan < 1 || req.ColSpan > 3 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Width must be between 1 and 3 columns."})
		return
	}
	if req.Size != "sm" && req.Size != "md" && req.Size != "lg" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Height must be one of: sm, md, lg."})
		return
	}

	tx := db.GetTx(ctx)

	starred, err := transactional.WidgetGroupRepository.FindStarredByWidgetId(tx, projectId, widgetId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update starred widget layout: %w", err))
		return
	}
	if starred == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Starred widget not found"})
		return
	}

	starred.ColSpan = req.ColSpan
	starred.Size = req.Size

	if err := transactional.WidgetGroupRepository.UpdateStarred(tx, starred); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update starred widget layout: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, starred)
}

func validateWidgetConfig(raw json.RawMessage) string {
	var cfg struct {
		Sources []struct {
			Name string `json:"name"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "Invalid widget configuration."
	}
	hasMetric := false
	for _, s := range cfg.Sources {
		if strings.TrimSpace(s.Name) != "" {
			hasMetric = true
			break
		}
	}
	if !hasMetric {
		return "Please select a Metric."
	}
	return ""
}

var WidgetController = widgetController{}
