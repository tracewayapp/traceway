package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
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
	Pagination    PaginationParams `json:"pagination"`
}

type ProfileSeriesRequest struct {
	FromDate        time.Time `json:"fromDate"`
	ToDate          time.Time `json:"toDate"`
	ServiceName     string    `json:"serviceName"`
	Type            string    `json:"type"`
	IntervalMinutes int       `json:"intervalMinutes"`
}

type ProfileFlameGraphRequest struct {
	FromDate    time.Time         `json:"fromDate"`
	ToDate      time.Time         `json:"toDate"`
	ServiceName string            `json:"serviceName"`
	Type        string            `json:"type"`
	Labels      map[string]string `json:"labels"`
}

type ProfileSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
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

	groups, total, err := repositories.ProfileRepository.FindGroupedByService(c, projectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection)
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

	points, err := repositories.ProfileRepository.GetSeries(c, projectId, request.ServiceName, request.Type, request.FromDate, request.ToDate, request.IntervalMinutes)
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

	rows, err := repositories.ProfileRepository.GetFlameGraph(c, projectId, request.ServiceName, request.Type, request.FromDate, request.ToDate, request.Labels)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading flame graph: %w", err))
		return
	}

	c.JSON(http.StatusOK, services.FoldFlameGraph(rows))
}

var ProfileController = profileController{}
