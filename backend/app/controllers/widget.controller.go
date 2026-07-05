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
	"github.com/tracewayapp/traceway/backend/app/repositories"

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

	group, err := repositories.WidgetGroupRepository.FindById(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to add widget: %w", err))
		return
	}
	if group == nil || group.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget group not found"})
		return
	}

	existing, err := repositories.WidgetGroupRepository.FindWidgetsByGroup(tx, groupId)
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
	id, err := repositories.WidgetGroupRepository.CreateWidget(tx, w)
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

	group, err := repositories.WidgetGroupRepository.FindById(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update widget: %w", err))
		return
	}
	if group == nil || group.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget group not found"})
		return
	}

	widget, err := repositories.WidgetGroupRepository.FindWidgetById(tx, widgetId)
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

	if err := repositories.WidgetGroupRepository.UpdateWidget(tx, widget); err != nil {
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

	group, err := repositories.WidgetGroupRepository.FindById(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reorder widgets: %w", err))
		return
	}
	if group == nil || group.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget group not found"})
		return
	}

	allWidgets, err := repositories.WidgetGroupRepository.FindWidgetsByGroup(tx, groupId)
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
		if err := repositories.WidgetGroupRepository.UpdateWidget(tx, w); err != nil {
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

	group, err := repositories.WidgetGroupRepository.FindById(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete widget: %w", err))
		return
	}
	if group == nil || group.ProjectId != projectId {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}

	widget, err := repositories.WidgetGroupRepository.FindWidgetById(tx, widgetId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete widget: %w", err))
		return
	}
	if widget == nil || widget.WidgetGroupId != groupId {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}

	if err := repositories.WidgetGroupRepository.DeleteWidget(tx, widgetId); err != nil {
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

	group, err := repositories.WidgetGroupRepository.FindById(tx, groupId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
		return
	}
	if group == nil || group.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget group not found"})
		return
	}

	widget, err := repositories.WidgetGroupRepository.FindWidgetById(tx, widgetId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
		return
	}
	if widget == nil || widget.WidgetGroupId != groupId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget not found"})
		return
	}

	widget.IsStarred = !widget.IsStarred
	widget.UpdatedAt = time.Now().UTC()

	if err := repositories.WidgetGroupRepository.UpdateWidget(tx, widget); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, widget)
}

func (c *widgetController) ListStarred(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)

	widgets, err := repositories.WidgetGroupRepository.FindStarredWidgetsByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list starred widgets: %w", err))
		return
	}

	if widgets == nil {
		widgets = []*models.WidgetGroupWidget{}
	}

	ctx.JSON(http.StatusOK, widgets)
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
