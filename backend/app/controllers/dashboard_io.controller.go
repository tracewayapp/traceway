package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	dashboardsvc "github.com/tracewayapp/traceway/backend/app/services/dashboards"
	"github.com/tracewayapp/traceway/backend/app/services/grafana"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

func exportDashboard(ctx *gin.Context, dashboard *models.Dashboard) (*models.DashboardExport, bool) {
	def := dashboardDefinitionOrError(ctx, dashboard)
	if def == nil {
		return nil, false
	}
	return &models.DashboardExport{
		SchemaVersion: dashboardsvc.SchemaVersion,
		Name:          dashboard.Name,
		Description:   dashboard.Description,
		Widgets:       def.Widgets,
	}, true
}

func (c *dashboardsController) Export(ctx *gin.Context) {
	tx := db.GetTx(ctx)

	dashboard := loadDashboardForUser(ctx, tx, false)
	if dashboard == nil {
		return
	}

	export, ok := exportDashboard(ctx, dashboard)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, export)
}

func (c *dashboardsController) ExportOrganization(ctx *gin.Context) {
	organizationId, err := strconv.Atoi(ctx.Query("organizationId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "organizationId query param is required"})
		return
	}

	tx := db.GetTx(ctx)

	userId := middleware.GetUserId(ctx)
	role, err := transactional.OrganizationRepository.GetUserRole(tx, organizationId, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve organization role: %w", err))
		return
	}
	if role == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	dashboards, err := transactional.DashboardRepository.FindByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list dashboards: %w", err))
		return
	}

	bundle := models.DashboardBundleExport{
		SchemaVersion: dashboardsvc.SchemaVersion,
		Dashboards:    []models.DashboardExport{},
	}
	for _, d := range dashboards {
		export, ok := exportDashboard(ctx, d)
		if !ok {
			return
		}
		bundle.Dashboards = append(bundle.Dashboards, *export)
	}

	ctx.JSON(http.StatusOK, bundle)
}

type ImportDashboardsRequest struct {
	OrganizationId    *int            `json:"organizationId"`
	Mode              string          `json:"mode"`
	Data              json.RawMessage `json:"data" binding:"required"`
	ApplyToProjectIds []uuid.UUID     `json:"applyToProjectIds"`
}

func parseImportedDashboards(raw json.RawMessage) ([]models.DashboardExport, string) {
	var bundle models.DashboardBundleExport
	if err := json.Unmarshal(raw, &bundle); err == nil && len(bundle.Dashboards) > 0 {
		if !dashboardsvc.SupportedSchemaVersion(bundle.SchemaVersion) {
			return nil, unsupportedSchemaVersionMessage()
		}
		return bundle.Dashboards, ""
	}

	var doc models.DashboardExport
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "The file is not valid dashboard JSON."
	}
	if len(doc.Widgets) == 0 && strings.TrimSpace(doc.Name) == "" {
		return nil, "The file contains no dashboards."
	}
	return []models.DashboardExport{doc}, ""
}

func (c *dashboardsController) Import(ctx *gin.Context) {
	limitRequestBody(ctx)
	var req ImportDashboardsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if req.Mode == "" {
		req.Mode = "create"
	}
	if req.Mode != "create" && req.Mode != "upsert" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "mode must be \"create\" or \"upsert\"."})
		return
	}

	docs, msg := parseImportedDashboards(req.Data)
	if msg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return
	}

	tx := db.GetTx(ctx)

	organizationId, ok := resolveTargetOrganization(ctx, tx, req.OrganizationId)
	if !ok {
		return
	}
	if !requireOrgWrite(ctx, tx, organizationId) {
		return
	}

	applyTo := req.ApplyToProjectIds
	if applyTo == nil {
		if projectIdStr := ctx.Query("projectId"); projectIdStr != "" {
			if projectId, err := uuid.Parse(projectIdStr); err == nil {
				applyTo = []uuid.UUID{projectId}
			}
		}
	}

	userId := middleware.GetUserId(ctx)
	var createdBy *int
	if userId > 0 {
		createdBy = &userId
	}

	created := 0
	updated := 0
	items := []DashboardListItem{}
	for i := range docs {
		doc := &docs[i]
		if !dashboardsvc.SupportedSchemaVersion(doc.SchemaVersion) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("Dashboard %d: %s", i+1, unsupportedSchemaVersionMessage())})
			return
		}
		name, msg := validateDashboardName(doc.Name)
		if msg != "" {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("Dashboard %d: %s", i+1, msg)})
			return
		}

		def := &models.DashboardDefinition{SchemaVersion: dashboardsvc.SchemaVersion, Widgets: doc.Widgets}
		if msg := validateDefinitionWidgets(def); msg != "" {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("Dashboard %q: %s", name, msg)})
			return
		}
		dashboardsvc.EnsureWidgetIds(def)
		definition, ok := marshalDashboardDefinition(ctx, def)
		if !ok {
			return
		}

		existing, err := transactional.DashboardRepository.FindByOrganization(tx, organizationId)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list dashboards: %w", err))
			return
		}

		var target *models.Dashboard
		if req.Mode == "upsert" {
			for _, d := range existing {
				if strings.EqualFold(d.Name, name) {
					target = d
					break
				}
			}
		}

		now := time.Now().UTC()
		if target != nil {
			oldDef := dashboardDefinitionOrError(ctx, target)
			if oldDef == nil {
				return
			}
			kept := map[string]bool{}
			for _, w := range def.Widgets {
				kept[w.Id] = true
			}
			for _, w := range oldDef.Widgets {
				if !kept[w.Id] {
					if err := transactional.DashboardRepository.DeleteStarredByWidget(tx, target.Id, w.Id); err != nil {
						ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to clean up starred widgets: %w", err))
						return
					}
				}
			}
			target.Description = doc.Description
			target.Definition = definition
			target.UpdatedAt = now
			if err := transactional.DashboardRepository.Update(tx, target); err != nil {
				if db.IsUniqueViolation(err) {
					ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("A dashboard named %q already exists.", name)})
					return
				}
				ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update dashboard: %w", err))
				return
			}
			updated++
			if !applyDashboardToProjects(ctx, tx, target, applyTo) {
				return
			}
			items = append(items, dashboardListItem(target))
			continue
		}

		importName := name
		for dashboardNameTaken(existing, importName, 0) {
			suffixed := importName + " (imported)"
			if len(suffixed) > maxDashboardNameLength {
				ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("A dashboard named %q already exists.", name)})
				return
			}
			importName = suffixed
		}

		dashboard := &models.Dashboard{
			OrganizationId: organizationId,
			Name:           importName,
			Description:    doc.Description,
			Definition:     definition,
			CreatedBy:      createdBy,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		id, err := transactional.DashboardRepository.Create(tx, dashboard)
		if err != nil {
			if db.IsUniqueViolation(err) {
				ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("A dashboard named %q already exists.", importName)})
				return
			}
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to import dashboard: %w", err))
			return
		}
		dashboard.Id = id
		created++
		if !applyDashboardToProjects(ctx, tx, dashboard, applyTo) {
			return
		}
		items = append(items, dashboardListItem(dashboard))
	}

	ctx.JSON(http.StatusCreated, gin.H{"dashboards": items, "created": created, "updated": updated})
}

type ImportGrafanaRequest struct {
	OrganizationId    *int            `json:"organizationId"`
	Grafana           json.RawMessage `json:"grafana" binding:"required"`
	ApplyToProjectIds []uuid.UUID     `json:"applyToProjectIds"`
}

func (c *dashboardsController) ImportGrafana(ctx *gin.Context) {
	limitRequestBody(ctx)
	var req ImportGrafanaRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)

	organizationId, ok := resolveTargetOrganization(ctx, tx, req.OrganizationId)
	if !ok {
		return
	}
	if !requireOrgWrite(ctx, tx, organizationId) {
		return
	}

	result, err := grafana.Convert(req.Grafana)
	if err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The file is not a valid Grafana dashboard export."})
		return
	}
	if len(result.Definition.Widgets) == 0 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":    "No panels could be converted from this Grafana dashboard.",
			"warnings": result.Warnings,
		})
		return
	}
	if msg := validateDefinitionWidgets(result.Definition); msg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg, "warnings": result.Warnings})
		return
	}

	definition, ok := marshalDashboardDefinition(ctx, result.Definition)
	if !ok {
		return
	}

	existing, err := transactional.DashboardRepository.FindByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list dashboards: %w", err))
		return
	}
	name := dashboardsvc.TruncateName(result.Name, maxDashboardNameLength)
	for dashboardNameTaken(existing, name, 0) {
		suffixed := name + " (imported)"
		if len(suffixed) > maxDashboardNameLength {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("A dashboard named %q already exists.", result.Name)})
			return
		}
		name = suffixed
	}

	userId := middleware.GetUserId(ctx)
	var createdBy *int
	if userId > 0 {
		createdBy = &userId
	}

	now := time.Now().UTC()
	dashboard := &models.Dashboard{
		OrganizationId: organizationId,
		Name:           name,
		Description:    result.Description,
		Definition:     definition,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	id, err := transactional.DashboardRepository.Create(tx, dashboard)
	if err != nil {
		if db.IsUniqueViolation(err) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A dashboard with this name already exists."})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to import Grafana dashboard: %w", err))
		return
	}
	dashboard.Id = id

	applyTo := req.ApplyToProjectIds
	if applyTo == nil {
		if projectIdStr := ctx.Query("projectId"); projectIdStr != "" {
			if projectId, err := uuid.Parse(projectIdStr); err == nil {
				applyTo = []uuid.UUID{projectId}
			}
		}
	}
	if !applyDashboardToProjects(ctx, tx, dashboard, applyTo) {
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"dashboard": dashboardListItem(dashboard),
		"warnings":  result.Warnings,
	})
}
