package controllers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/v2/app/sdkconfig"
	dashboardsvc "github.com/tracewayapp/traceway/backend/v2/app/services/dashboards"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type dashboardTemplateController struct{}

type DashboardTemplateListItem struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	WidgetCount int    `json:"widgetCount"`
}

func (c *dashboardTemplateController) List(ctx *gin.Context) {
	tx := db.GetTx(ctx)

	templates, err := transactional.DashboardTemplateRepository.FindAll(tx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list dashboard templates: %w", err))
		return
	}

	search := strings.ToLower(strings.TrimSpace(ctx.Query("search")))
	category := strings.TrimSpace(ctx.Query("category"))

	items := []DashboardTemplateListItem{}
	for _, t := range templates {
		if category != "" && t.Category != category {
			continue
		}
		if search != "" {
			haystack := strings.ToLower(t.Key + " " + t.Name + " " + t.Description + " " + t.Category)
			if !strings.Contains(haystack, search) {
				continue
			}
		}
		widgetCount := 0
		def, err := dashboardsvc.ParseDefinition(t.Definition)
		if err != nil {
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to parse definition of dashboard template %s: %w", t.Key, err))
		} else {
			widgetCount = len(def.Widgets)
		}
		items = append(items, DashboardTemplateListItem{
			Key:         t.Key,
			Name:        t.Name,
			Description: t.Description,
			Category:    t.Category,
			WidgetCount: widgetCount,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"templates": items})
}

func uniqueDashboardName(existing []*models.Dashboard, base string, projectName string) string {
	taken := func(name string) bool { return dashboardNameTaken(existing, name, 0) }
	return dashboardsvc.UniqueName(taken, base, projectName, maxDashboardNameLength)
}

var errTemplateInvalid = errors.New("invalid dashboard template")

var errDashboardNameTaken = errors.New("dashboard name already exists")

// installDashboardTemplateTx names the copy after projectName when the template's name is taken in the organization;
// an empty projectName (a gallery install for any number of projects) numbers it instead.
func installDashboardTemplateTx(tx *sql.Tx, template *models.DashboardTemplate, organizationId int, createdBy *int, projectName string) (*models.Dashboard, error) {
	def, err := dashboardsvc.ParseDefinition(template.Definition)
	if err != nil {
		return nil, traceway.NewStackTraceErrorf("failed to parse template %s definition: %w (%w)", template.Key, err, errTemplateInvalid)
	}
	for i := range def.Widgets {
		def.Widgets[i].WidgetType = strings.TrimSpace(def.Widgets[i].WidgetType)
		if def.Widgets[i].WidgetType == "" {
			def.Widgets[i].WidgetType = "line_chart"
		}
		if !dashboardsvc.IsAllowedWidgetType(def.Widgets[i].WidgetType) {
			return nil, traceway.NewStackTraceErrorf("template %s contains unsupported widget type %s (%w)", template.Key, def.Widgets[i].WidgetType, errTemplateInvalid)
		}
	}
	dashboardsvc.EnsureWidgetIds(def)
	definition, err := dashboardsvc.MarshalDefinition(def)
	if err != nil {
		return nil, traceway.NewStackTraceErrorf("failed to marshal template %s definition: %w (%w)", template.Key, err, errTemplateInvalid)
	}

	existing, err := transactional.DashboardRepository.FindByOrganization(tx, organizationId)
	if err != nil {
		return nil, traceway.NewStackTraceErrorf("failed to list dashboards: %w", err)
	}
	name := uniqueDashboardName(existing, template.Name, projectName)

	templateKey := template.Key
	now := time.Now().UTC()
	dashboard := &models.Dashboard{
		OrganizationId: organizationId,
		Name:           name,
		Description:    template.Description,
		Definition:     definition,
		TemplateKey:    &templateKey,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	id, err := transactional.DashboardRepository.Create(tx, dashboard)
	if err != nil {
		if db.IsUniqueViolation(err) {
			return nil, errDashboardNameTaken
		}
		return nil, traceway.NewStackTraceErrorf("failed to install template %s: %w", template.Key, err)
	}
	dashboard.Id = id
	return dashboard, nil
}

func installDashboardTemplate(ctx *gin.Context, tx *sql.Tx, template *models.DashboardTemplate, organizationId int, projectName string) *models.Dashboard {
	userId := middleware.GetUserId(ctx)
	var createdBy *int
	if userId > 0 {
		createdBy = &userId
	}

	dashboard, err := installDashboardTemplateTx(tx, template, organizationId, createdBy, projectName)
	if err != nil {
		if errors.Is(err, errDashboardNameTaken) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "A dashboard with this name already exists."})
			return nil
		}
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return nil
	}
	return dashboard
}

// PopulateDashboardsOnCreate installs a new project's default dashboards on the paths a person creates it from (the
// project page, registration, OAuth signup, the bootstrap file). React Native projects have no Dashboards page and
// their SDK sends no app.* metrics yet, so they get nothing there; the batch path keeps installing theirs.
func PopulateDashboardsOnCreate(tx *sql.Tx, project *models.Project, createdBy *int) error {
	if project.Framework == "react-native" {
		return nil
	}
	return populateDefaultDashboards(tx, project, createdBy)
}

func populateDefaultDashboards(tx *sql.Tx, project *models.Project, createdBy *int) error {
	if project.OrganizationId == nil {
		return nil
	}
	keys := defaultTemplateKeysForFramework(project.Framework)
	if len(keys) == 0 {
		return nil
	}
	// Dashboard names are unique per organization: the lock EnsureBrowserDefault takes keeps two projects created
	// at the same moment from both picking the same free name and failing the second creation.
	if !db.IsSQLite() {
		if _, err := tx.Exec("SELECT pg_advisory_xact_lock(824738, $1)", *project.OrganizationId); err != nil {
			return err
		}
	}

	existing, err := transactional.DashboardRepository.FindAssignmentsByProject(tx, project.Id)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}

	position := 0
	for _, key := range keys {
		template, err := transactional.DashboardTemplateRepository.FindByKey(tx, key)
		if err != nil {
			return err
		}
		if template == nil {
			traceway.CaptureException(fmt.Errorf("default dashboard template %s is not seeded", key))
			continue
		}

		dashboard, err := installDashboardTemplateTx(tx, template, *project.OrganizationId, createdBy, project.Name)
		if err != nil {
			if errors.Is(err, errTemplateInvalid) {
				traceway.CaptureException(err)
				continue
			}
			return err
		}
		if err := transactional.DashboardRepository.CreateAssignment(tx, &models.ProjectDashboard{
			ProjectId:   project.Id,
			DashboardId: dashboard.Id,
			Position:    position,
			CreatedAt:   time.Now().UTC(),
		}); err != nil {
			return err
		}
		position++
	}
	return nil
}

type InstallTemplateRequest struct {
	OrganizationId *int        `json:"organizationId"`
	ProjectIds     []uuid.UUID `json:"projectIds"`
}

func (c *dashboardTemplateController) Install(ctx *gin.Context) {
	var req InstallTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}

	tx := db.GetTx(ctx)

	template, err := transactional.DashboardTemplateRepository.FindByKey(tx, ctx.Param("key"))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find dashboard template: %w", err))
		return
	}
	if template == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	organizationId, ok := resolveTargetOrganization(ctx, tx, req.OrganizationId)
	if !ok {
		return
	}
	if !requireOrgWrite(ctx, tx, organizationId) {
		return
	}

	dashboard := installDashboardTemplate(ctx, tx, template, organizationId, "")
	if dashboard == nil {
		return
	}

	applyTo := req.ProjectIds
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

// mobileFrameworks get the Mobile Performance template. React Native is a mobile app
// although its SDK is JavaScript. Flutter has its own template: its SDK measures no
// ANRs or out-of-memory kills, and an unhandled Dart error is not a crash.
var mobileFrameworks = map[string]bool{"react-native": true, "ios": true, "android": true}

func defaultTemplateKeysForFramework(framework string) []string {
	goFrameworks := map[string]bool{
		"gin": true, "fiber": true, "chi": true,
		"fasthttp": true, "stdlib": true, "custom": true,
	}
	switch {
	case framework == "opentelemetry":
		return []string{"traceway-otel-agent"}
	case goFrameworks[framework]:
		return []string{"golang"}
	case sdkconfig.IsBrowserFramework(framework):
		return []string{"browser-performance"}
	case framework == "flutter":
		return []string{"flutter-performance"}
	case mobileFrameworks[framework]:
		return []string{"mobile-performance"}
	default:
		return nil
	}
}

func (c *dashboardsController) PopulateDefaults(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	tx := db.GetTx(ctx)

	existing, err := transactional.DashboardRepository.FindAssignmentsByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list dashboard assignments: %w", err))
		return
	}
	if len(existing) > 0 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Project already has dashboards."})
		return
	}

	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find project: %w", err))
		return
	}
	if project == nil || project.OrganizationId == nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Project has no organization."})
		return
	}
	if !requireOrgWrite(ctx, tx, *project.OrganizationId) {
		return
	}

	keys := defaultTemplateKeysForFramework(project.Framework)
	if len(keys) == 0 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "No default dashboards are available for this project's framework."})
		return
	}

	position := 0
	items := []DashboardListItem{}
	for _, key := range keys {
		template, err := transactional.DashboardTemplateRepository.FindByKey(tx, key)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to find dashboard template %s: %w", key, err))
			return
		}
		if template == nil {
			traceway.CaptureException(fmt.Errorf("default dashboard template %s is not seeded", key))
			continue
		}

		dashboard := installDashboardTemplate(ctx, tx, template, *project.OrganizationId, project.Name)
		if dashboard == nil {
			return
		}
		if err := transactional.DashboardRepository.CreateAssignment(tx, &models.ProjectDashboard{
			ProjectId:   projectId,
			DashboardId: dashboard.Id,
			Position:    position,
			CreatedAt:   time.Now().UTC(),
		}); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to assign default dashboard: %w", err))
			return
		}
		position++
		items = append(items, dashboardListItem(dashboard))
	}

	ctx.JSON(http.StatusCreated, gin.H{"dashboards": items})
}

var DashboardTemplateController = dashboardTemplateController{}
