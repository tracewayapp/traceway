package controllers

import (
	"net/http"
	"time"

	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type sessionController struct{}

type SessionAttributeFilter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SessionSearchRequest struct {
	FromDate         time.Time                `json:"fromDate"`
	ToDate           time.Time                `json:"toDate"`
	OrderBy          string                   `json:"orderBy"`
	SortDirection    string                   `json:"sortDirection"`
	Search           string                   `json:"search"`
	AttributeFilters []SessionAttributeFilter `json:"attributeFilters"`
	Pagination       PaginationParams         `json:"pagination"`
}

func (s sessionController) FindAllSessions(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request SessionSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filters := make([]telemetry.SessionAttributeFilter, 0, len(request.AttributeFilters))
	for _, f := range request.AttributeFilters {
		if f.Key == "" {
			continue
		}
		filters = append(filters, telemetry.SessionAttributeFilter{Key: f.Key, Value: f.Value})
	}

	span := traceway.StartSpan(c, "loading sessions")
	sessions, total, err := telemetry.SessionRepository.FindAll(c, projectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection, request.Search, filters)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading sessions: %w", err))
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.Session]{
		Data: sessions,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

var SessionController = sessionController{}
