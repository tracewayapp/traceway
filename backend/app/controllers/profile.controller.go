package controllers

import (
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/profiling"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/services"
	traceway "go.tracewayapp.com"
)

type profileController struct{}

type ProfileGroupedRequest struct {
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Search        string           `json:"search"`
	Pagination    PaginationParams `json:"pagination"`
}

type ProfileSeriesRequest struct {
	FromDate        time.Time         `json:"fromDate"`
	ToDate          time.Time         `json:"toDate"`
	ServiceName     string            `json:"serviceName"`
	Type            string            `json:"type"`
	IsGauge         bool              `json:"isGauge"`
	IntervalMinutes int               `json:"intervalMinutes"`
	Labels          map[string]string `json:"labels"`
	AppVersion      string            `json:"appVersion"`
	ServerName      string            `json:"serverName"`
	Normalize       bool              `json:"normalize"`
}

type ProfileSeriesBreakdownRequest struct {
	FromDate        time.Time         `json:"fromDate"`
	ToDate          time.Time         `json:"toDate"`
	ServiceName     string            `json:"serviceName"`
	Type            string            `json:"type"`
	IsGauge         bool              `json:"isGauge"`
	IntervalMinutes int               `json:"intervalMinutes"`
	Labels          map[string]string `json:"labels"`
	AppVersion      string            `json:"appVersion"`
	ServerName      string            `json:"serverName"`
	Normalize       bool              `json:"normalize"`
	Dimension       string            `json:"dimension"`
	Limit           int               `json:"limit"`
}

type ProfileDimensionsRequest struct {
	FromDate    time.Time `json:"fromDate"`
	ToDate      time.Time `json:"toDate"`
	ServiceName string    `json:"serviceName"`
	Type        string    `json:"type"`
}

type ProfileFlameGraphRequest struct {
	FromDate    time.Time         `json:"fromDate"`
	ToDate      time.Time         `json:"toDate"`
	ServiceName string            `json:"serviceName"`
	Type        string            `json:"type"`
	Unit        string            `json:"unit"`
	IsGauge     bool              `json:"isGauge"`
	Labels      map[string]string `json:"labels"`
	AppVersion  string            `json:"appVersion"`
	ServerName  string            `json:"serverName"`
}

type ProfileTypesRequest struct {
	FromDate    time.Time `json:"fromDate"`
	ToDate      time.Time `json:"toDate"`
	ServiceName string    `json:"serviceName"`
}

type ProfileTopFunctionsRequest struct {
	FromDate    time.Time         `json:"fromDate"`
	ToDate      time.Time         `json:"toDate"`
	ServiceName string            `json:"serviceName"`
	Type        string            `json:"type"`
	IsGauge     bool              `json:"isGauge"`
	Labels      map[string]string `json:"labels"`
	Limit       int               `json:"limit"`
	AppVersion  string            `json:"appVersion"`
	ServerName  string            `json:"serverName"`
}

type ProfileLabelsRequest struct {
	FromDate    time.Time `json:"fromDate"`
	ToDate      time.Time `json:"toDate"`
	ServiceName string    `json:"serviceName"`
	Type        string    `json:"type"`
}

type ProfileSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type ProfileSeriesGroupResponse struct {
	Key    string               `json:"key"`
	Points []ProfileSeriesPoint `json:"points"`
}

func (p profileController) FindGroupedByService(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ProfileGroupedRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	search := strings.TrimSpace(request.Search)
	if utf8.RuneCountInString(search) > 200 {
		search = ""
	}

	groups, total, err := repositories.ProfileRepository.FindGroupedByService(c, projectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection, search)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading grouped profiles: %w", err))
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.ProfileGroup]{
		Data: groups,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (p profileController) GetSeries(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ProfileSeriesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if request.ServiceName == "" || request.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName and type are required"})
		return
	}
	if !validateProfileLabelFilters(c, projectId, request.Labels) {
		return
	}
	if !validateProfileCohort(c, request.AppVersion, request.ServerName) {
		return
	}

	points, err := repositories.ProfileRepository.GetSeries(c, projectId, request.ServiceName, request.Type, request.FromDate, request.ToDate, request.IntervalMinutes, request.IsGauge, request.Labels, request.AppVersion, request.ServerName, request.Normalize)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading profile series: %w", err))
		return
	}

	out := make([]ProfileSeriesPoint, 0, len(points))
	for _, pt := range points {
		out = append(out, ProfileSeriesPoint{Timestamp: pt.Timestamp, Value: pt.Value})
	}
	c.JSON(http.StatusOK, out)
}

func (p profileController) DiscoverTypes(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ProfileTypesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if request.ServiceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName is required"})
		return
	}

	typesFrom := request.FromDate
	if !request.ToDate.IsZero() {
		typesFrom = request.ToDate.AddDate(0, 0, -30)
		if request.FromDate.Before(typesFrom) {
			typesFrom = request.FromDate
		}
	}

	types, err := repositories.ProfileRepository.DiscoverTypes(c, projectId, request.ServiceName, typesFrom, request.ToDate)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error discovering profile types: %w", err))
		return
	}
	if types == nil {
		types = []models.ProfileTypeInfo{}
	}
	c.JSON(http.StatusOK, types)
}

func (p profileController) GetFlameGraph(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ProfileFlameGraphRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if request.ServiceName == "" || request.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName and type are required"})
		return
	}

	if !validateProfileLabelFilters(c, projectId, request.Labels) {
		return
	}
	if !validateProfileCohort(c, request.AppVersion, request.ServerName) {
		return
	}

	rows, err := repositories.ProfileRepository.GetFlameGraph(c, projectId, request.ServiceName, request.Type, request.FromDate, request.ToDate, request.Labels, request.IsGauge, request.AppVersion, request.ServerName)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading flame graph: %w", err))
		return
	}

	c.JSON(http.StatusOK, services.FoldFlameGraph(rows))
}

func (p profileController) GetTopFunctions(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ProfileTopFunctionsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if request.ServiceName == "" || request.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName and type are required"})
		return
	}
	if !validateProfileLabelFilters(c, projectId, request.Labels) {
		return
	}
	if !validateProfileCohort(c, request.AppVersion, request.ServerName) {
		return
	}

	limit := request.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := repositories.ProfileRepository.GetFlameGraph(c, projectId, request.ServiceName, request.Type, request.FromDate, request.ToDate, request.Labels, request.IsGauge, request.AppVersion, request.ServerName)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading top functions: %w", err))
		return
	}

	c.JSON(http.StatusOK, services.TopFunctions(rows, limit))
}

func (p profileController) DownloadPprof(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ProfileFlameGraphRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if request.ServiceName == "" || request.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName and type are required"})
		return
	}
	if !validateProfileLabelFilters(c, projectId, request.Labels) {
		return
	}
	if !validateProfileCohort(c, request.AppVersion, request.ServerName) {
		return
	}

	rows, err := repositories.ProfileRepository.GetFlameGraph(c, projectId, request.ServiceName, request.Type, request.FromDate, request.ToDate, request.Labels, request.IsGauge, request.AppVersion, request.ServerName)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading profile for export: %w", err))
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no profile data for the selected range"})
		return
	}

	data, err := services.BuildPprof(request.Type, request.Unit, rows)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error building pprof export: %w", err))
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\""+pprofFilename(request.ServiceName, request.Type)+"\"")
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func pprofFilename(serviceName, profileType string) string {
	safe := func(s string) string {
		out := make([]rune, 0, len(s))
		for _, r := range s {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
				out = append(out, r)
			default:
				out = append(out, '_')
			}
		}
		return string(out)
	}
	return safe(serviceName) + "-" + safe(profileType) + ".pprof"
}

func validateProfileLabelFilters(c *gin.Context, projectId uuid.UUID, labels map[string]string) bool {
	if len(labels) == 0 {
		return true
	}
	var additional []string
	if proj := cache.ProjectCache.GetById(projectId); proj != nil {
		additional = proj.ProfileLabelAllowlist
	}
	allowed := profiling.LabelAllowKeys(additional)
	for key := range labels {
		if utf8.RuneCountInString(key) > 100 || !slices.Contains(allowed, key) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "label filter key is not in the project's profile label allowlist"})
			return false
		}
	}
	return true
}

func validateProfileCohort(c *gin.Context, appVersion, serverName string) bool {
	if utf8.RuneCountInString(appVersion) > 200 || utf8.RuneCountInString(serverName) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "appVersion and serverName must be at most 200 characters"})
		return false
	}
	return true
}

func (p profileController) GetSeriesBreakdown(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ProfileSeriesBreakdownRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if request.ServiceName == "" || request.Type == "" || request.Dimension == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName, type and dimension are required"})
		return
	}
	if !validateProfileLabelFilters(c, projectId, request.Labels) {
		return
	}
	if !validateProfileCohort(c, request.AppVersion, request.ServerName) {
		return
	}

	groups, err := repositories.ProfileRepository.GetSeriesBreakdown(c, projectId, request.ServiceName, request.Type, request.FromDate, request.ToDate, request.IntervalMinutes, request.IsGauge, request.Labels, request.AppVersion, request.ServerName, request.Normalize, request.Dimension, request.Limit)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading profile series breakdown: %w", err))
		return
	}

	out := make([]ProfileSeriesGroupResponse, 0, len(groups))
	for _, g := range groups {
		points := make([]ProfileSeriesPoint, 0, len(g.Points))
		for _, pt := range g.Points {
			points = append(points, ProfileSeriesPoint{Timestamp: pt.Timestamp, Value: pt.Value})
		}
		out = append(out, ProfileSeriesGroupResponse{Key: g.Key, Points: points})
	}
	c.JSON(http.StatusOK, out)
}

func (p profileController) DiscoverDimensions(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ProfileDimensionsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if request.ServiceName == "" || request.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName and type are required"})
		return
	}

	appVersions, serverNames, err := repositories.ProfileRepository.DiscoverDimensions(c, projectId, request.ServiceName, request.Type, request.FromDate, request.ToDate)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error discovering profile dimensions: %w", err))
		return
	}
	if appVersions == nil {
		appVersions = []string{}
	}
	if serverNames == nil {
		serverNames = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"appVersions": appVersions, "serverNames": serverNames})
}

func (p profileController) DiscoverLabels(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ProfileLabelsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if request.ServiceName == "" || request.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName and type are required"})
		return
	}

	var additional []string
	if proj := cache.ProjectCache.GetById(projectId); proj != nil {
		additional = proj.ProfileLabelAllowlist
	}

	labels, err := repositories.ProfileRepository.DiscoverLabels(c, projectId, request.ServiceName, request.Type, request.FromDate, request.ToDate, profiling.LabelAllowKeys(additional))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error discovering profile labels: %w", err))
		return
	}

	c.JSON(http.StatusOK, labels)
}

var ProfileController = profileController{}
