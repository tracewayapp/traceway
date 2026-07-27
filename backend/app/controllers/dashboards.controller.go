package controllers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	dashboardsvc "github.com/tracewayapp/traceway/backend/app/services/dashboards"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type dashboardsController struct{}

const maxDashboardNameLength = 100

const maxDashboardBodyBytes = 5 << 20

func limitRequestBody(ctx *gin.Context) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxDashboardBodyBytes)
}

func definitionProvided(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

var widgetTypeErrorMessage = "Widget type must be one of: " + strings.Join(dashboardsvc.AllowedWidgetTypes, ", ") + "."

func unsupportedSchemaVersionMessage() string {
	return fmt.Sprintf("Unsupported dashboard schemaVersion. This server supports schemaVersion %d.", dashboardsvc.SchemaVersion)
}

type DashboardListItem struct {
	Id             int     `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	OrganizationId int     `json:"organizationId"`
	TemplateKey    *string `json:"templateKey"`
}

type DashboardWidgetWithStar struct {
	Id         string          `json:"id"`
	Title      string          `json:"title"`
	WidgetType string          `json:"widgetType"`
	Config     json.RawMessage `json:"config"`
	IsStarred  bool            `json:"isStarred"`
}

type DashboardResponse struct {
	Id                int                       `json:"id"`
	OrganizationId    int                       `json:"organizationId"`
	Name              string                    `json:"name"`
	Description       string                    `json:"description"`
	TemplateKey       *string                   `json:"templateKey"`
	CreatedAt         time.Time                 `json:"createdAt"`
	UpdatedAt         time.Time                 `json:"updatedAt"`
	AppliedProjectIds []uuid.UUID               `json:"appliedProjectIds"`
	Widgets           []DashboardWidgetWithStar `json:"widgets"`
}

func dashboardListItem(d *models.Dashboard) DashboardListItem {
	return DashboardListItem{
		Id:             d.Id,
		Name:           d.Name,
		Description:    d.Description,
		OrganizationId: d.OrganizationId,
		TemplateKey:    d.TemplateKey,
	}
}

func validateDashboardName(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "Name is required."
	}
	if utf8.RuneCountInString(name) > maxDashboardNameLength {
		return "", "Name must be 100 characters or fewer."
	}
	return name, ""
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

func validateDefinitionWidgets(def *models.DashboardDefinition) string {
	if len(def.Widgets) > dashboardsvc.MaxWidgetsPerDashboard {
		return "A dashboard can have at most 100 widgets."
	}
	for i := range def.Widgets {
		w := &def.Widgets[i]
		w.Title = strings.TrimSpace(w.Title)
		if w.Title == "" {
			return "Every widget needs a title."
		}
		w.WidgetType = strings.TrimSpace(w.WidgetType)
		if w.WidgetType == "" {
			w.WidgetType = "line_chart"
		}
		if !dashboardsvc.IsAllowedWidgetType(w.WidgetType) {
			return widgetTypeErrorMessage
		}
		if len(w.Config) == 0 {
			w.Config = json.RawMessage(`{}`)
		}
		if len(w.Config) > dashboardsvc.MaxWidgetConfigBytes {
			return "Widget configuration is too large."
		}
		if msg := validateWidgetConfig(w.Config); msg != "" {
			return msg
		}
	}
	return ""
}

func loadDashboardForUser(ctx *gin.Context, tx *sql.Tx, requireWrite bool) *models.Dashboard {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dashboard id"})
		return nil
	}

	dashboard, err := transactional.DashboardRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load dashboard: %w", err))
		return nil
	}
	if dashboard == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Dashboard not found"})
		return nil
	}

	userId := middleware.GetUserId(ctx)
	role, err := transactional.OrganizationRepository.GetUserRole(tx, dashboard.OrganizationId, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve organization role: %w", err))
		return nil
	}
	if role == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Dashboard not found"})
		return nil
	}
	if requireWrite && role == "readonly" {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "You have read-only access to this organization"})
		return nil
	}
	return dashboard
}

func requireOrgWrite(ctx *gin.Context, tx *sql.Tx, organizationId int) bool {
	userId := middleware.GetUserId(ctx)
	role, err := transactional.OrganizationRepository.GetUserRole(tx, organizationId, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve organization role: %w", err))
		return false
	}
	if role == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return false
	}
	if role == "readonly" {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "You have read-only access to this organization"})
		return false
	}
	return true
}

func requireProjectWrite(ctx *gin.Context, tx *sql.Tx, projectId uuid.UUID) bool {
	userId := middleware.GetUserId(ctx)
	role, err := transactional.ProjectRepository.GetEffectiveRole(tx, projectId, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve project role: %w", err))
		return false
	}
	if role == "" || role == "readonly" {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "You do not have write access to this project"})
		return false
	}
	return true
}

func parseDashboardDefinition(ctx *gin.Context, raw json.RawMessage) *models.DashboardDefinition {
	def, err := dashboardsvc.ParseDefinition(raw)
	if err != nil {
		if errors.Is(err, dashboardsvc.ErrUnsupportedSchemaVersion) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": unsupportedSchemaVersionMessage()})
			return nil
		}
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid dashboard definition JSON."})
		return nil
	}
	if msg := validateDefinitionWidgets(def); msg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return nil
	}
	dashboardsvc.EnsureWidgetIds(def)
	return def
}

func marshalDashboardDefinition(ctx *gin.Context, def *models.DashboardDefinition) ([]byte, bool) {
	raw, err := dashboardsvc.MarshalDefinition(def)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to marshal dashboard definition: %w", err))
		return nil, false
	}
	return raw, true
}

func dashboardDefinitionOrError(ctx *gin.Context, dashboard *models.Dashboard) *models.DashboardDefinition {
	def, err := dashboardsvc.ParseDefinition(dashboard.Definition)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to parse definition of dashboard %d: %w", dashboard.Id, err))
		return nil
	}
	return def
}

func (c *dashboardsController) List(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)

	dashboards, err := transactional.DashboardRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list dashboards: %w", err))
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

	items := []DashboardListItem{}
	for _, d := range dashboards {
		items = append(items, dashboardListItem(d))
	}

	ctx.JSON(http.StatusOK, gin.H{
		"dashboards":          items,
		"framework":           framework,
		"canPopulateDefaults": len(items) == 0 && len(defaultTemplateKeysForFramework(framework)) > 0,
	})
}

type LibraryDashboard struct {
	Id                int         `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	TemplateKey       *string     `json:"templateKey"`
	WidgetCount       int         `json:"widgetCount"`
	AppliedProjectIds []uuid.UUID `json:"appliedProjectIds"`
	UpdatedAt         time.Time   `json:"updatedAt"`
}

type LibraryOrganization struct {
	Id         int                `json:"id"`
	Name       string             `json:"name"`
	Role       string             `json:"role"`
	Dashboards []LibraryDashboard `json:"dashboards"`
}

func (c *dashboardsController) Library(ctx *gin.Context) {
	userId := middleware.GetUserId(ctx)
	tx := db.GetTx(ctx)

	orgs, err := transactional.OrganizationRepository.FindByUserId(tx, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organizations: %w", err))
		return
	}

	result := []LibraryOrganization{}
	for _, org := range orgs {
		role, err := transactional.OrganizationRepository.GetUserRole(tx, org.Id, userId)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve organization role: %w", err))
			return
		}

		dashboards, err := transactional.DashboardRepository.FindByOrganization(tx, org.Id)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list dashboards for organization %d: %w", org.Id, err))
			return
		}
		assignments, err := transactional.DashboardRepository.FindAssignmentsByOrganization(tx, org.Id)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list dashboard assignments for organization %d: %w", org.Id, err))
			return
		}

		applied := map[int][]uuid.UUID{}
		for _, a := range assignments {
			applied[a.DashboardId] = append(applied[a.DashboardId], a.ProjectId)
		}

		libOrg := LibraryOrganization{Id: org.Id, Name: org.Name, Role: role, Dashboards: []LibraryDashboard{}}
		for _, d := range dashboards {
			def, err := dashboardsvc.ParseDefinition(d.Definition)
			widgetCount := 0
			if err != nil {
				traceway.CaptureException(traceway.NewStackTraceErrorf("failed to parse definition of dashboard %d: %w", d.Id, err))
			} else {
				widgetCount = len(def.Widgets)
			}
			projectIds := applied[d.Id]
			if projectIds == nil {
				projectIds = []uuid.UUID{}
			}
			libOrg.Dashboards = append(libOrg.Dashboards, LibraryDashboard{
				Id:                d.Id,
				Name:              d.Name,
				Description:       d.Description,
				TemplateKey:       d.TemplateKey,
				WidgetCount:       widgetCount,
				AppliedProjectIds: projectIds,
				UpdatedAt:         d.UpdatedAt,
			})
		}
		result = append(result, libOrg)
	}

	ctx.JSON(http.StatusOK, gin.H{"organizations": result})
}

func resolveTargetOrganization(ctx *gin.Context, tx *sql.Tx, explicitOrgId *int) (int, bool) {
	if explicitOrgId != nil {
		return *explicitOrgId, true
	}
	projectIdStr := ctx.Query("projectId")
	if projectIdStr != "" {
		if projectId, err := uuid.Parse(projectIdStr); err == nil {
			project, err := transactional.ProjectRepository.FindById(tx, projectId)
			if err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find project: %w", err))
				return 0, false
			}
			if project != nil && project.OrganizationId != nil {
				return *project.OrganizationId, true
			}
		}
	}
	ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "An organization is required."})
	return 0, false
}

func dashboardNameTaken(dashboards []*models.Dashboard, name string, excludeId int) bool {
	for _, d := range dashboards {
		if d.Id != excludeId && strings.EqualFold(d.Name, name) {
			return true
		}
	}
	return false
}

func applyDashboardToProjects(ctx *gin.Context, tx *sql.Tx, dashboard *models.Dashboard, projectIds []uuid.UUID) bool {
	for _, projectId := range projectIds {
		project, err := transactional.ProjectRepository.FindById(tx, projectId)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find project: %w", err))
			return false
		}
		if project == nil || project.OrganizationId == nil || *project.OrganizationId != dashboard.OrganizationId {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "All projects must belong to the dashboard's organization."})
			return false
		}
		if !requireProjectWrite(ctx, tx, projectId) {
			return false
		}

		assignments, err := transactional.DashboardRepository.FindAssignmentsByProject(tx, projectId)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list dashboard assignments: %w", err))
			return false
		}
		nextPosition := 0
		for _, a := range assignments {
			if a.DashboardId == dashboard.Id {
				nextPosition = -1
				break
			}
			if a.Position >= nextPosition {
				nextPosition = a.Position + 1
			}
		}
		if nextPosition < 0 {
			continue
		}
		if err := transactional.DashboardRepository.CreateAssignment(tx, &models.ProjectDashboard{
			ProjectId:   projectId,
			DashboardId: dashboard.Id,
			Position:    nextPosition,
			CreatedAt:   time.Now().UTC(),
		}); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to assign dashboard: %w", err))
			return false
		}
	}
	return true
}

type CreateDashboardRequest struct {
	OrganizationId    *int            `json:"organizationId"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	Definition        json.RawMessage `json:"definition"`
	ApplyToProjectIds []uuid.UUID     `json:"applyToProjectIds"`
}

func (c *dashboardsController) Create(ctx *gin.Context) {
	limitRequestBody(ctx)
	var req CreateDashboardRequest
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

	name, msg := validateDashboardName(req.Name)
	if msg != "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return
	}

	existing, err := transactional.DashboardRepository.FindByOrganization(tx, organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check duplicate dashboard name: %w", err))
		return
	}
	if dashboardNameTaken(existing, name, 0) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A dashboard with this name already exists."})
		return
	}

	def := parseDashboardDefinition(ctx, req.Definition)
	if def == nil {
		return
	}
	definition, ok := marshalDashboardDefinition(ctx, def)
	if !ok {
		return
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
		Description:    req.Description,
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
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create dashboard: %w", err))
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

	ctx.JSON(http.StatusCreated, dashboardListItem(dashboard))
}

func (c *dashboardsController) Get(ctx *gin.Context) {
	tx := db.GetTx(ctx)

	dashboard := loadDashboardForUser(ctx, tx, false)
	if dashboard == nil {
		return
	}

	def := dashboardDefinitionOrError(ctx, dashboard)
	if def == nil {
		return
	}

	starredIds := map[string]bool{}
	if projectIdStr := ctx.Query("projectId"); projectIdStr != "" {
		if projectId, err := uuid.Parse(projectIdStr); err == nil {
			starred, err := transactional.DashboardRepository.FindStarredByProjectAndDashboard(tx, projectId, dashboard.Id)
			if err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load starred widgets: %w", err))
				return
			}
			for _, s := range starred {
				starredIds[s.WidgetId] = true
			}
		}
	}

	widgets := []DashboardWidgetWithStar{}
	for _, w := range def.Widgets {
		widgets = append(widgets, DashboardWidgetWithStar{
			Id:         w.Id,
			Title:      w.Title,
			WidgetType: w.WidgetType,
			Config:     w.Config,
			IsStarred:  starredIds[w.Id],
		})
	}

	assignments, err := transactional.DashboardRepository.FindAssignmentsByDashboard(tx, dashboard.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list dashboard assignments: %w", err))
		return
	}
	appliedProjectIds := []uuid.UUID{}
	for _, a := range assignments {
		appliedProjectIds = append(appliedProjectIds, a.ProjectId)
	}

	ctx.JSON(http.StatusOK, DashboardResponse{
		Id:                dashboard.Id,
		OrganizationId:    dashboard.OrganizationId,
		Name:              dashboard.Name,
		Description:       dashboard.Description,
		TemplateKey:       dashboard.TemplateKey,
		CreatedAt:         dashboard.CreatedAt,
		UpdatedAt:         dashboard.UpdatedAt,
		AppliedProjectIds: appliedProjectIds,
		Widgets:           widgets,
	})
}

type UpdateDashboardRequest struct {
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Definition  json.RawMessage `json:"definition"`
}

func (c *dashboardsController) Update(ctx *gin.Context) {
	limitRequestBody(ctx)
	var req UpdateDashboardRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)

	dashboard := loadDashboardForUser(ctx, tx, true)
	if dashboard == nil {
		return
	}

	if req.Name != nil {
		name, msg := validateDashboardName(*req.Name)
		if msg != "" {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
			return
		}
		if !strings.EqualFold(name, dashboard.Name) {
			existing, err := transactional.DashboardRepository.FindByOrganization(tx, dashboard.OrganizationId)
			if err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check duplicate dashboard name: %w", err))
				return
			}
			if dashboardNameTaken(existing, name, dashboard.Id) {
				ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A dashboard with this name already exists."})
				return
			}
		}
		dashboard.Name = name
	}
	if req.Description != nil {
		dashboard.Description = *req.Description
	}

	if definitionProvided(req.Definition) {
		oldDef := dashboardDefinitionOrError(ctx, dashboard)
		if oldDef == nil {
			return
		}
		def := parseDashboardDefinition(ctx, req.Definition)
		if def == nil {
			return
		}
		definition, ok := marshalDashboardDefinition(ctx, def)
		if !ok {
			return
		}

		kept := map[string]bool{}
		for _, w := range def.Widgets {
			kept[w.Id] = true
		}
		for _, w := range oldDef.Widgets {
			if !kept[w.Id] {
				if err := transactional.DashboardRepository.DeleteStarredByWidget(tx, dashboard.Id, w.Id); err != nil {
					ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to clean up starred widgets: %w", err))
					return
				}
			}
		}
		dashboard.Definition = definition
	}

	dashboard.UpdatedAt = time.Now().UTC()
	if err := transactional.DashboardRepository.Update(tx, dashboard); err != nil {
		if db.IsUniqueViolation(err) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A dashboard with this name already exists."})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update dashboard: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, dashboardListItem(dashboard))
}

func (c *dashboardsController) Delete(ctx *gin.Context) {
	tx := db.GetTx(ctx)

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dashboard id"})
		return
	}

	dashboard, err := transactional.DashboardRepository.FindById(tx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete dashboard: %w", err))
		return
	}
	if dashboard == nil {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}

	userId := middleware.GetUserId(ctx)
	role, err := transactional.OrganizationRepository.GetUserRole(tx, dashboard.OrganizationId, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve organization role: %w", err))
		return
	}
	if role == "" {
		ctx.JSON(http.StatusOK, gin.H{"deleted": true})
		return
	}
	if role == "readonly" {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "You have read-only access to this organization"})
		return
	}

	if err := transactional.DashboardRepository.DeleteStarredByDashboard(tx, id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete dashboard starred widgets: %w", err))
		return
	}
	if err := transactional.DashboardRepository.DeleteAssignmentsByDashboard(tx, id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete dashboard assignments: %w", err))
		return
	}
	if err := transactional.DashboardRepository.Delete(tx, id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete dashboard: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

type ApplyDashboardRequest struct {
	ProjectIds []uuid.UUID `json:"projectIds" binding:"required"`
}

func (c *dashboardsController) Apply(ctx *gin.Context) {
	var req ApplyDashboardRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)

	dashboard := loadDashboardForUser(ctx, tx, true)
	if dashboard == nil {
		return
	}

	current, err := transactional.DashboardRepository.FindAssignmentsByDashboard(tx, dashboard.Id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list dashboard assignments: %w", err))
		return
	}

	desired := map[uuid.UUID]bool{}
	for _, projectId := range req.ProjectIds {
		desired[projectId] = true
	}

	toAdd := []uuid.UUID{}
	for projectId := range desired {
		found := false
		for _, a := range current {
			if a.ProjectId == projectId {
				found = true
				break
			}
		}
		if !found {
			toAdd = append(toAdd, projectId)
		}
	}

	for _, a := range current {
		if desired[a.ProjectId] {
			continue
		}
		if !requireProjectWrite(ctx, tx, a.ProjectId) {
			return
		}
		if err := transactional.DashboardRepository.DeleteStarredByProjectAndDashboard(tx, a.ProjectId, dashboard.Id); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to clean up starred widgets: %w", err))
			return
		}
		if err := transactional.DashboardRepository.DeleteAssignment(tx, a.ProjectId, dashboard.Id); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to unassign dashboard: %w", err))
			return
		}
	}

	if !applyDashboardToProjects(ctx, tx, dashboard, toAdd) {
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"applied": true})
}

func (c *dashboardsController) Unapply(ctx *gin.Context) {
	tx := db.GetTx(ctx)

	dashboard := loadDashboardForUser(ctx, tx, false)
	if dashboard == nil {
		return
	}

	projectId, err := uuid.Parse(ctx.Param("projectId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project id"})
		return
	}
	if !requireProjectWrite(ctx, tx, projectId) {
		return
	}

	if err := transactional.DashboardRepository.DeleteStarredByProjectAndDashboard(tx, projectId, dashboard.Id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to clean up starred widgets: %w", err))
		return
	}
	if err := transactional.DashboardRepository.DeleteAssignment(tx, projectId, dashboard.Id); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to unassign dashboard: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"removed": true})
}

type CopyDashboardRequest struct {
	OrganizationId    *int        `json:"organizationId"`
	Name              string      `json:"name"`
	ApplyToProjectIds []uuid.UUID `json:"applyToProjectIds"`
}

func (c *dashboardsController) Copy(ctx *gin.Context) {
	var req CopyDashboardRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)

	source := loadDashboardForUser(ctx, tx, false)
	if source == nil {
		return
	}

	targetOrgId := source.OrganizationId
	if req.OrganizationId != nil {
		targetOrgId = *req.OrganizationId
	}
	if !requireOrgWrite(ctx, tx, targetOrgId) {
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = source.Name
	}
	name = dashboardsvc.TruncateName(name, maxDashboardNameLength)

	existing, err := transactional.DashboardRepository.FindByOrganization(tx, targetOrgId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check duplicate dashboard name: %w", err))
		return
	}
	for dashboardNameTaken(existing, name, 0) {
		suffixed := name + " (copy)"
		if len(suffixed) > maxDashboardNameLength {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A dashboard with this name already exists."})
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
	copyRow := &models.Dashboard{
		OrganizationId: targetOrgId,
		Name:           name,
		Description:    source.Description,
		Definition:     source.Definition,
		TemplateKey:    source.TemplateKey,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	id, err := transactional.DashboardRepository.Create(tx, copyRow)
	if err != nil {
		if db.IsUniqueViolation(err) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A dashboard with this name already exists."})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to copy dashboard: %w", err))
		return
	}
	copyRow.Id = id

	if !applyDashboardToProjects(ctx, tx, copyRow, req.ApplyToProjectIds) {
		return
	}

	ctx.JSON(http.StatusCreated, dashboardListItem(copyRow))
}

type ReorderDashboardsRequest struct {
	DashboardIds []int `json:"dashboardIds" binding:"required,min=1"`
}

func (c *dashboardsController) Reorder(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var req ReorderDashboardsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)

	assignments, err := transactional.DashboardRepository.FindAssignmentsByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reorder dashboards: %w", err))
		return
	}

	byDashboardId := make(map[int]*models.ProjectDashboard, len(assignments))
	for _, a := range assignments {
		byDashboardId[a.DashboardId] = a
	}

	if len(req.DashboardIds) != len(assignments) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The dashboard list is out of date. Please refresh and try again."})
		return
	}
	seen := make(map[int]bool, len(req.DashboardIds))
	for _, id := range req.DashboardIds {
		if byDashboardId[id] == nil || seen[id] {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The dashboard list is out of date. Please refresh and try again."})
			return
		}
		seen[id] = true
	}

	for position, id := range req.DashboardIds {
		a := byDashboardId[id]
		if a.Position == position {
			continue
		}
		a.Position = position
		if err := transactional.DashboardRepository.UpdateAssignment(tx, a); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to reorder dashboards: %w", err))
			return
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"reordered": true})
}

var DashboardsController = dashboardsController{}
