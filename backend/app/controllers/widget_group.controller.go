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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type widgetGroupController struct{}

func (c *widgetGroupController) List(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)

	list, err := transactional.WidgetGroupRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list widget groups: %w", err))
		return
	}

	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find project: %w", err))
		return
	}

	framework := ""
	if project != nil {
		framework = project.Framework
	}

	ctx.JSON(http.StatusOK, gin.H{
		"widgetGroups":        list,
		"framework":           framework,
		"canPopulateDefaults": len(list) == 0,
	})
}

func (c *widgetGroupController) PopulateDefaults(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)

	existing, err := transactional.WidgetGroupRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list widget groups: %w", err))
		return
	}
	if len(existing) > 0 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Project already has widget groups."})
		return
	}

	if err := ensureDefaultWidgetGroups(tx, projectId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create default widget groups: %w", err))
		return
	}

	list, err := transactional.WidgetGroupRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list widget groups: %w", err))
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"widgetGroups": list})
}

type CreateWidgetGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (c *widgetGroupController) Create(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var req CreateWidgetGroupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name is required."})
		return
	}
	if len(req.Name) > 12 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name must be 12 characters or fewer."})
		return
	}

	tx := db.GetTx(ctx)

	existing, err := transactional.WidgetGroupRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check duplicate widget group name: %w", err))
		return
	}
	for _, g := range existing {
		if strings.EqualFold(g.Name, req.Name) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A group with this name already exists."})
			return
		}
	}

	userId := middleware.GetUserId(ctx)
	var createdBy *int
	if userId > 0 {
		createdBy = &userId
	}

	g := &models.WidgetGroup{
		ProjectId:   projectId,
		Name:        req.Name,
		Description: req.Description,
		IsDefault:   false,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	id, err := transactional.WidgetGroupRepository.Create(tx, g)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create widget group: %w", err))
		return
	}
	g.Id = id

	ctx.JSON(http.StatusCreated, g)
}

func (c *widgetGroupController) GetWithWidgets(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget group id"})
		return
	}

	tx := db.GetTx(ctx)

	group, err := transactional.WidgetGroupRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to get widget group: %w", err))
		return
	}
	if group == nil || group.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget group not found"})
		return
	}

	widgets, err := transactional.WidgetGroupRepository.FindWidgetsByGroupWithStar(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to get widget group widgets: %w", err))
		return
	}

	widgetSlice := []models.WidgetGroupWidgetWithStar{}
	for _, w := range widgets {
		widgetSlice = append(widgetSlice, *w)
	}

	ctx.JSON(http.StatusOK, &models.WidgetGroupWithWidgets{
		WidgetGroup: *group,
		Widgets:     widgetSlice,
	})
}

func (c *widgetGroupController) Update(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget group id"})
		return
	}

	var req CreateWidgetGroupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name is required."})
		return
	}
	if len(req.Name) > 12 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name must be 12 characters or fewer."})
		return
	}

	tx := db.GetTx(ctx)

	existing, err := transactional.WidgetGroupRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update widget group: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Widget group not found"})
		return
	}

	allGroups, err := transactional.WidgetGroupRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check duplicate widget group name: %w", err))
		return
	}
	for _, g := range allGroups {
		if g.Id != id && strings.EqualFold(g.Name, req.Name) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A group with this name already exists."})
			return
		}
	}

	existing.Name = req.Name
	existing.Description = req.Description
	existing.UpdatedAt = time.Now().UTC()

	if err := transactional.WidgetGroupRepository.Update(tx, existing); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update widget group: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, existing)
}

func (c *widgetGroupController) Delete(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid widget group id"})
		return
	}

	tx := db.GetTx(ctx)

	existing, err := transactional.WidgetGroupRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete widget group: %w", err))
		return
	}
	if existing == nil || existing.ProjectId != projectId {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}

	// Same transaction handles children + parent: delete child widgets first,
	// then the group itself. Don't lean on the FK cascade — explicit deletes
	// are easier to audit and survive future schema migrations that might
	// drop or alter the cascade rule.
	if err := transactional.WidgetGroupRepository.DeleteStarredByGroup(tx, id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete widget group starred widgets: %w", err))
		return
	}

	if err := transactional.WidgetGroupRepository.DeleteWidgetsByGroup(tx, id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete widget group widgets: %w", err))
		return
	}

	if err := transactional.WidgetGroupRepository.Delete(tx, id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete widget group: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

func ensureDefaultWidgetGroups(tx *sql.Tx, projectId uuid.UUID) error {
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		return err
	}

	jsFrameworks := map[string]bool{
		"react": true, "svelte": true, "vuejs": true,
		"nextjs": true, "nestjs": true, "express": true, "remix": true,
	}
	otelFrameworks := map[string]bool{
		"opentelemetry": true,
	}
	isJS := project != nil && jsFrameworks[project.Framework]
	isOtel := project != nil && otelFrameworks[project.Framework]

	type widgetDef struct {
		title      string
		name       string
		widgetType string // optional; empty string falls back to "line_chart"
	}
	type groupDef struct {
		name    string
		widgets []widgetDef
	}

	var groupDefs []groupDef

	if isOtel {
		// Defaults track what `traceway-otel-agent` emits out-of-the-box. The
		// `process` scraper is opt-in (TRACEWAY_PROCESS_NAMES) as of v0.5.0
		// and intentionally excluded here — seeding it produced empty tabs.
		groupDefs = append(groupDefs,
			groupDef{
				name: "System",
				widgets: []widgetDef{
					{title: "CPU Utilization", name: "system.cpu.utilization"},
					{title: "Memory Utilization", name: "system.memory.utilization"},
					// area_chart: filled region reads naturally as "consumed memory".
					{title: "Memory Usage", name: "system.memory.usage", widgetType: "area_chart"},
					{title: "Load Avg (1m)", name: "system.cpu.load_average.1m"},
					{title: "Load Avg (5m)", name: "system.cpu.load_average.5m"},
					{title: "Load Avg (15m)", name: "system.cpu.load_average.15m"},
				},
			},
			groupDef{
				name: "Storage",
				widgets: []widgetDef{
					{title: "Filesystem Utilization", name: "system.filesystem.utilization"},
					// area_chart: same reasoning as Memory Usage — filled = used disk.
					{title: "Filesystem Usage", name: "system.filesystem.usage", widgetType: "area_chart"},
					{title: "Disk I/O", name: "system.disk.io"},
					{title: "Disk IOPS", name: "system.disk.operations"},
					{title: "Disk I/O Time", name: "system.disk.io_time"},
				},
			},
			groupDef{
				name: "Network",
				widgets: []widgetDef{
					{title: "Network I/O", name: "system.network.io"},
					{title: "Network Packets", name: "system.network.packets"},
					{title: "Network Errors", name: "system.network.errors"},
					{title: "Open Connections", name: "system.network.connections"},
				},
			},
		)
	} else {
		if !isJS {
			groupDefs = append(groupDefs, groupDef{
				name: "Application",
				widgets: []widgetDef{
					{title: "Go Routines", name: "go.go_routines"},
					{title: "Heap Objects", name: "go.heap_objects"},
					{title: "GC Cycles", name: "go.num_gc"},
					{title: "GC Pause", name: "go.gc_pause"},
				},
			})
		}

		groupDefs = append(groupDefs,
			groupDef{
				name: "Stats",
				widgets: []widgetDef{
					{title: "Memory Usage", name: "mem.used"},
					{title: "Total Memory", name: "mem.total"},
				},
			},
			groupDef{
				name: "CPU / Mem",
				widgets: []widgetDef{
					{title: "CPU Usage", name: "cpu.used_pcnt"},
					{title: "Memory Usage", name: "mem.used"},
				},
			},
		)
	}

	now := time.Now().UTC()
	for _, gd := range groupDefs {
		g := &models.WidgetGroup{
			ProjectId: projectId,
			Name:      gd.name,
			IsDefault: true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		groupId, err := transactional.WidgetGroupRepository.Create(tx, g)
		if err != nil {
			return err
		}

		for i, w := range gd.widgets {
			config := json.RawMessage(`{"sources":[{"type":"metric","name":"` + w.name + `","aggregation":"avg"}]}`)
			widgetType := w.widgetType
			if widgetType == "" {
				widgetType = "line_chart"
			}
			widget := &models.WidgetGroupWidget{
				WidgetGroupId: groupId,
				Title:         w.title,
				WidgetType:    widgetType,
				Config:        config,
				Position:      i,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if _, err := transactional.WidgetGroupRepository.CreateWidget(tx, widget); err != nil {
				return err
			}
		}
	}

	return nil
}

var WidgetGroupController = widgetGroupController{}
