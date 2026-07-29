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
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type metricQueryController struct{}

type MetricQueryRequest struct {
	Queries         []MetricQueryItem `json:"queries" binding:"required,min=1"`
	From            time.Time         `json:"from" binding:"required"`
	To              time.Time         `json:"to" binding:"required"`
	IntervalMinutes int               `json:"intervalMinutes"`
}

type MetricQueryItem struct {
	Name        string            `json:"name" binding:"required"`
	Aggregation string            `json:"aggregation"`
	TagFilters  map[string]string `json:"tagFilters"`
	GroupBy     string            `json:"groupBy"`
}

type MetricQueryResponse struct {
	Results []MetricQueryResult `json:"results"`
}

type MetricQueryResult struct {
	Name   string                              `json:"name"`
	Unit   string                              `json:"unit"`
	Series map[string][]models.TimeSeriesPoint `json:"series"`
}

func (c *metricQueryController) Query(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var req MetricQueryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.IntervalMinutes <= 0 {
		req.IntervalMinutes = calculateIntervalMinutes(req.To.Sub(req.From))
	}

	unitMap := make(map[string]string)
	registry, regErr := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.MetricRegistry, error) {
		return transactional.MetricRegistryRepository.FindByProject(tx, projectId)
	})
	if regErr != nil {
		traceway.CaptureException(fmt.Errorf("failed to load metric registry for query: %w", regErr))
	}
	if regErr == nil {
		for _, r := range registry {
			unitMap[r.Name] = r.Unit
		}
	}

	var results []MetricQueryResult
	for _, q := range req.Queries {
		agg := q.Aggregation
		if agg == "" {
			agg = "avg"
		}

		spanName := fmt.Sprintf("query metric: %s agg=%s", q.Name, agg)
		if q.GroupBy != "" {
			spanName += " group=" + q.GroupBy
		}
		if len(q.TagFilters) > 0 {
			spanName += fmt.Sprintf(" filters=%d", len(q.TagFilters))
		}
		span := traceway.StartSpan(ctx, spanName)
		series, err := telemetry.MetricPointRepository.QueryTimeSeries(
			ctx, projectId, q.Name, req.From, req.To,
			req.IntervalMinutes, agg, q.TagFilters, q.GroupBy,
		)
		span.End()
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query metric %s: %w", q.Name, err))
			return
		}

		resultUnit := unitMap[q.Name]
		if agg == "count" {
			resultUnit = "count"
		}

		results = append(results, MetricQueryResult{
			Name:   q.Name,
			Unit:   resultUnit,
			Series: series,
		})
	}

	ctx.JSON(http.StatusOK, MetricQueryResponse{Results: results})
}

type DiscoverResponse struct {
	Metrics []models.DiscoveredMetric `json:"metrics"`
}

func (c *metricQueryController) Discover(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	from := time.Now().AddDate(0, 0, -7)
	to := time.Now()

	if fromStr := ctx.Query("from"); fromStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = parsed
		}
	}
	if toStr := ctx.Query("to"); toStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = parsed
		}
	}

	span := traceway.StartSpan(ctx, "discover metrics")
	discovered, err := telemetry.MetricPointRepository.DiscoverMetrics(ctx, projectId, from, to)
	span.End()
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to discover metrics: %w", err))
		return
	}

	registry, regErr := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.MetricRegistry, error) {
		return transactional.MetricRegistryRepository.FindByProject(tx, projectId)
	})

	if regErr != nil {
		traceway.CaptureException(fmt.Errorf("failed to load metric registry for discover: %w", regErr))
	}
	if regErr == nil {
		regMap := make(map[string]*models.MetricRegistry, len(registry))
		for _, r := range registry {
			regMap[r.Name] = r
		}
		for i := range discovered {
			if reg, ok := regMap[discovered[i].Name]; ok {
				discovered[i].MetricType = reg.MetricType
				discovered[i].Unit = reg.Unit
			}
		}
	}

	ctx.JSON(http.StatusOK, DiscoverResponse{Metrics: discovered})
}

const discoverOrgMaxProjects = 25

type OrgDiscoveredMetric struct {
	Name       string      `json:"name"`
	TagKeys    []string    `json:"tagKeys"`
	MetricType string      `json:"metricType,omitempty"`
	Unit       string      `json:"unit,omitempty"`
	ProjectIds []uuid.UUID `json:"projectIds"`
}

func (c *metricQueryController) DiscoverOrg(ctx *gin.Context) {
	organizationId, err := strconv.Atoi(ctx.Query("organizationId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "organizationId query param is required"})
		return
	}

	userId := middleware.GetUserId(ctx)
	projects, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Project, error) {
		role, err := transactional.OrganizationRepository.GetUserRole(tx, organizationId, userId)
		if err != nil {
			return nil, err
		}
		if role == "" {
			return nil, nil
		}
		return transactional.ProjectRepository.FindByOrganizationId(tx, organizationId)
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list organization projects: %w", err))
		return
	}
	if projects == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}
	truncated := false
	if len(projects) > discoverOrgMaxProjects {
		projects = projects[:discoverOrgMaxProjects]
		truncated = true
	}

	from := time.Now().AddDate(0, 0, -7)
	to := time.Now()

	merged := map[string]*OrgDiscoveredMetric{}
	order := []string{}
	for _, project := range projects {
		span := traceway.StartSpan(ctx, fmt.Sprintf("discover metrics: project %s", project.Id))
		discovered, err := telemetry.MetricPointRepository.DiscoverMetrics(ctx, project.Id, from, to)
		span.End()
		if err != nil {
			traceway.CaptureException(fmt.Errorf("failed to discover metrics for project %s: %w", project.Id, err))
			continue
		}
		for _, m := range discovered {
			entry := merged[m.Name]
			if entry == nil {
				entry = &OrgDiscoveredMetric{Name: m.Name, TagKeys: m.TagKeys, MetricType: m.MetricType, Unit: m.Unit, ProjectIds: []uuid.UUID{}}
				merged[m.Name] = entry
				order = append(order, m.Name)
			}
			entry.ProjectIds = append(entry.ProjectIds, project.Id)
			if entry.Unit == "" {
				entry.Unit = m.Unit
			}
			if entry.MetricType == "" {
				entry.MetricType = m.MetricType
			}
		}
	}

	metrics := make([]OrgDiscoveredMetric, 0, len(order))
	for _, name := range order {
		metrics = append(metrics, *merged[name])
	}

	ctx.JSON(http.StatusOK, gin.H{"metrics": metrics, "truncated": truncated})
}

func (c *metricQueryController) DiscoverTags(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	name := ctx.Query("name")
	key := ctx.Query("key")
	if name == "" || key == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "name and key query params are required"})
		return
	}

	from := time.Now().AddDate(0, 0, -7)
	to := time.Now()

	span := traceway.StartSpan(ctx, fmt.Sprintf("discover tag values: %s.%s", name, key))
	values, err := telemetry.MetricPointRepository.DiscoverTagValues(ctx, projectId, name, key, from, to)
	span.End()
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to discover tag values: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"values": values})
}

func (c *metricQueryController) UpdateRegistry(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		MetricType  string `json:"metricType"`
		Unit        string `json:"unit"`
		Description string `json:"description"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.MetricType != "" && req.MetricType != "gauge" && req.MetricType != "counter" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "metricType must be 'gauge' or 'counter'"})
		return
	}

	updated, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.MetricRegistry, error) {
		entry, err := transactional.MetricRegistryRepository.FindByProjectAndName(tx, projectId, req.Name)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			return nil, nil
		}
		if req.MetricType != "" {
			entry.MetricType = req.MetricType
		}
		if req.Unit != "" {
			entry.Unit = req.Unit
		}
		entry.Description = req.Description
		if err := transactional.MetricRegistryRepository.Update(tx, entry); err != nil {
			return nil, err
		}
		return entry, nil
	})

	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update metric registry: %w", err))
		return
	}
	if updated == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "metric not found in registry"})
		return
	}

	ctx.JSON(http.StatusOK, updated)
}

var MetricQueryController = metricQueryController{}
