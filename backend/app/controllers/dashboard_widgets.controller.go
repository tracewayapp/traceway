package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	dashboardsvc "github.com/tracewayapp/traceway/backend/app/services/dashboards"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type AddWidgetRequest struct {
	Title      string          `json:"title"`
	WidgetType string          `json:"widgetType" binding:"required"`
	Config     json.RawMessage `json:"config"`
}

func (c *dashboardsController) AddWidget(ctx *gin.Context) {
	limitRequestBody(ctx)
	var req AddWidgetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.Config == nil {
		req.Config = json.RawMessage(`{}`)
	}
	if len(req.Config) > dashboardsvc.MaxWidgetConfigBytes {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Widget configuration is too large."})
		return
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
	req.WidgetType = strings.TrimSpace(req.WidgetType)
	if !dashboardsvc.IsAllowedWidgetType(req.WidgetType) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": widgetTypeErrorMessage})
		return
	}

	tx := db.GetTx(ctx)

	dashboard := loadDashboardForUser(ctx, tx, true)
	if dashboard == nil {
		return
	}
	def := dashboardDefinitionOrError(ctx, dashboard)
	if def == nil {
		return
	}
	if len(def.Widgets) >= dashboardsvc.MaxWidgetsPerDashboard {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A dashboard can have at most 100 widgets."})
		return
	}

	used := make(map[string]bool, len(def.Widgets))
	for _, w := range def.Widgets {
		used[w.Id] = true
	}
	id := dashboardsvc.NewWidgetId()
	for used[id] {
		id = dashboardsvc.NewWidgetId()
	}

	widget := models.DashboardWidget{
		Id:         id,
		Title:      req.Title,
		WidgetType: req.WidgetType,
		Config:     req.Config,
	}
	def.Widgets = append(def.Widgets, widget)

	if !saveDashboardDefinition(ctx, tx, dashboard, def) {
		return
	}

	ctx.JSON(http.StatusCreated, widget)
}

func (c *dashboardsController) UpdateWidget(ctx *gin.Context) {
	limitRequestBody(ctx)
	var req AddWidgetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.Config != nil {
		if len(req.Config) > dashboardsvc.MaxWidgetConfigBytes {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Widget configuration is too large."})
			return
		}
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
	req.WidgetType = strings.TrimSpace(req.WidgetType)
	if !dashboardsvc.IsAllowedWidgetType(req.WidgetType) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": widgetTypeErrorMessage})
		return
	}

	tx := db.GetTx(ctx)

	dashboard := loadDashboardForUser(ctx, tx, true)
	if dashboard == nil {
		return
	}
	def := dashboardDefinitionOrError(ctx, dashboard)
	if def == nil {
		return
	}

	index := dashboardsvc.FindWidget(def, ctx.Param("wid"))
	if index < 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget not found"})
		return
	}

	def.Widgets[index].Title = req.Title
	def.Widgets[index].WidgetType = req.WidgetType
	if req.Config != nil {
		def.Widgets[index].Config = req.Config
	}

	if !saveDashboardDefinition(ctx, tx, dashboard, def) {
		return
	}

	ctx.JSON(http.StatusOK, def.Widgets[index])
}

func (c *dashboardsController) DeleteWidget(ctx *gin.Context) {
	tx := db.GetTx(ctx)

	dashboard := loadDashboardForUser(ctx, tx, true)
	if dashboard == nil {
		return
	}
	def := dashboardDefinitionOrError(ctx, dashboard)
	if def == nil {
		return
	}

	widgetId := ctx.Param("wid")
	index := dashboardsvc.FindWidget(def, widgetId)
	if index < 0 {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}

	def.Widgets = append(def.Widgets[:index], def.Widgets[index+1:]...)

	if err := transactional.DashboardRepository.DeleteStarredByWidget(tx, dashboard.Id, widgetId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete starred widget: %w", err))
		return
	}
	if !saveDashboardDefinition(ctx, tx, dashboard, def) {
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

type ReorderWidgetsRequest struct {
	WidgetIds []string `json:"widgetIds" binding:"required,min=1"`
}

func (c *dashboardsController) ReorderWidgets(ctx *gin.Context) {
	var req ReorderWidgetsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)

	dashboard := loadDashboardForUser(ctx, tx, true)
	if dashboard == nil {
		return
	}
	def := dashboardDefinitionOrError(ctx, dashboard)
	if def == nil {
		return
	}

	byId := make(map[string]models.DashboardWidget, len(def.Widgets))
	for _, w := range def.Widgets {
		byId[w.Id] = w
	}

	if len(req.WidgetIds) != len(def.Widgets) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The widget list is out of date. Please refresh and try again."})
		return
	}
	seen := make(map[string]bool, len(req.WidgetIds))
	reordered := make([]models.DashboardWidget, 0, len(def.Widgets))
	for _, id := range req.WidgetIds {
		w, ok := byId[id]
		if !ok || seen[id] {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The widget list is out of date. Please refresh and try again."})
			return
		}
		seen[id] = true
		reordered = append(reordered, w)
	}
	def.Widgets = reordered

	if !saveDashboardDefinition(ctx, tx, dashboard, def) {
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"reordered": true})
}

func saveDashboardDefinition(ctx *gin.Context, tx *sql.Tx, dashboard *models.Dashboard, def *models.DashboardDefinition) bool {
	definition, ok := marshalDashboardDefinition(ctx, def)
	if !ok {
		return false
	}
	dashboard.Definition = definition
	dashboard.UpdatedAt = time.Now().UTC()
	if err := transactional.DashboardRepository.Update(tx, dashboard); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update dashboard definition: %w", err))
		return false
	}
	return true
}

func (c *dashboardsController) ToggleStar(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)

	dashboard := loadDashboardForUser(ctx, tx, false)
	if dashboard == nil {
		return
	}

	assignment, err := transactional.DashboardRepository.FindAssignment(tx, projectId, dashboard.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check dashboard assignment: %w", err))
		return
	}
	if assignment == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Dashboard is not applied to this project"})
		return
	}

	def := dashboardDefinitionOrError(ctx, dashboard)
	if def == nil {
		return
	}
	widgetId := ctx.Param("wid")
	index := dashboardsvc.FindWidget(def, widgetId)
	if index < 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget not found"})
		return
	}

	starred, err := transactional.DashboardRepository.FindStarred(tx, projectId, dashboard.Id, widgetId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
		return
	}

	if starred != nil {
		if err := transactional.DashboardRepository.DeleteStarred(tx, projectId, dashboard.Id, widgetId); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"id": widgetId, "isStarred": false})
		return
	}

	allStarred, err := transactional.DashboardRepository.FindStarredByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
		return
	}

	var cfg struct {
		ColSpan int    `json:"colSpan"`
		Size    string `json:"size"`
	}
	_ = json.Unmarshal(def.Widgets[index].Config, &cfg)
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

	if err := transactional.DashboardRepository.CreateStarred(tx, &models.StarredDashboardWidget{
		ProjectId:   projectId,
		DashboardId: dashboard.Id,
		WidgetId:    widgetId,
		Position:    nextPosition,
		ColSpan:     cfg.ColSpan,
		Size:        cfg.Size,
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to toggle star: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"id": widgetId, "isStarred": true})
}

type StarredWidgetResponse struct {
	Id           int             `json:"id"`
	DashboardId  int             `json:"dashboardId"`
	WidgetId     string          `json:"widgetId"`
	Title        string          `json:"title"`
	WidgetType   string          `json:"widgetType"`
	Config       json.RawMessage `json:"config"`
	HomePosition int             `json:"homePosition"`
	HomeColSpan  int             `json:"homeColSpan"`
	HomeSize     string          `json:"homeSize"`
}

func (c *dashboardsController) ListStarred(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)

	starred, err := transactional.DashboardRepository.FindStarredByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list starred widgets: %w", err))
		return
	}

	dashboards, err := transactional.DashboardRepository.FindStarredDashboardsByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load starred dashboards: %w", err))
		return
	}

	widgetsByDashboard := map[int]map[string]models.DashboardWidget{}
	for _, d := range dashboards {
		def, err := dashboardsvc.ParseDefinition(d.Definition)
		if err != nil {
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to parse definition of dashboard %d: %w", d.Id, err))
			continue
		}
		widgets := make(map[string]models.DashboardWidget, len(def.Widgets))
		for _, w := range def.Widgets {
			widgets[w.Id] = w
		}
		widgetsByDashboard[d.Id] = widgets
	}

	response := []StarredWidgetResponse{}
	for _, s := range starred {
		widget, ok := widgetsByDashboard[s.DashboardId][s.WidgetId]
		if !ok {
			continue
		}
		response = append(response, StarredWidgetResponse{
			Id:           s.Id,
			DashboardId:  s.DashboardId,
			WidgetId:     s.WidgetId,
			Title:        widget.Title,
			WidgetType:   widget.WidgetType,
			Config:       widget.Config,
			HomePosition: s.Position,
			HomeColSpan:  s.ColSpan,
			HomeSize:     s.Size,
		})
	}

	ctx.JSON(http.StatusOK, response)
}

type ReorderStarredRequest struct {
	Ids []int `json:"ids" binding:"required,min=1"`
}

func (c *dashboardsController) ReorderStarred(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var req ReorderStarredRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)

	starred, err := transactional.DashboardRepository.FindStarredByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reorder starred widgets: %w", err))
		return
	}

	byId := make(map[int]*models.StarredDashboardWidget, len(starred))
	for _, s := range starred {
		byId[s.Id] = s
	}

	if len(req.Ids) != len(starred) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The widget list is out of date. Please refresh and try again."})
		return
	}
	seen := make(map[int]bool, len(req.Ids))
	for _, id := range req.Ids {
		if byId[id] == nil || seen[id] {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The widget list is out of date. Please refresh and try again."})
			return
		}
		seen[id] = true
	}

	for position, id := range req.Ids {
		s := byId[id]
		if s.Position == position {
			continue
		}
		s.Position = position
		if err := transactional.DashboardRepository.UpdateStarred(tx, s); err != nil {
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

func (c *dashboardsController) UpdateStarredLayout(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid starred widget id"})
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

	starred, err := transactional.DashboardRepository.FindStarredById(tx, projectId, id)
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
	if err := transactional.DashboardRepository.UpdateStarred(tx, starred); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update starred widget layout: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, starred)
}
