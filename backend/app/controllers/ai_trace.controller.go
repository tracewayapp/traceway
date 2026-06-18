package controllers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/storage"
	traceway "go.tracewayapp.com"
)

type aiTraceController struct{}

type aiTraceDetailRequest struct {
	RecordedAt *time.Time `json:"recordedAt"`
}

type AiTraceSearchRequest struct {
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
	Search        string           `json:"search"`
	RootFilter    string           `json:"rootFilter"`
}

type AiTraceInstancesRequest struct {
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
}

type AiTraceInstancesResponse struct {
	Data       []models.AiTrace           `json:"data"`
	Stats      *models.AiTraceDetailStats `json:"stats"`
	Pagination Pagination                 `json:"pagination"`
}

type AiTraceDetailResponse struct {
	AiTrace      *models.AiTrace `json:"aiTrace"`
	Conversation json.RawMessage `json:"conversation,omitempty"`
}

func (a aiTraceController) FindGroupedByTraceName(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request AiTraceSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stats, total, err := repositories.AiTraceRepository.FindGroupedByTraceName(c, projectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection, request.Search, request.RootFilter)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading ai trace stats: %w", err))
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.AiTraceStats]{
		Data: stats,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (a aiTraceController) FindByTraceName(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	rawTraceName := c.Query("traceName")
	if rawTraceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "traceName is required"})
		return
	}

	traceName, err := url.PathUnescape(rawTraceName)
	if err != nil {
		traceName = rawTraceName
	}

	var request AiTraceInstancesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	traces, total, err := repositories.AiTraceRepository.FindByTraceName(c, projectId, traceName, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading ai traces: %w", err))
		return
	}

	stats, err := repositories.AiTraceRepository.GetTraceNameStats(c, projectId, traceName, request.FromDate, request.ToDate)
	if err != nil {
		stats = nil
	}

	c.JSON(http.StatusOK, AiTraceInstancesResponse{
		Data:  traces,
		Stats: stats,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (a aiTraceController) GetAiTraceDetail(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	traceId, err := uuid.Parse(c.Param("traceId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid traceId"})
		return
	}

	var request aiTraceDetailRequest
	_ = c.ShouldBindJSON(&request)

	aiTrace, err := repositories.AiTraceRepository.FindById(c, projectId, traceId, request.RecordedAt)
	if aiTrace == nil && err == nil && request.RecordedAt != nil {
		aiTrace, err = repositories.AiTraceRepository.FindById(c, projectId, traceId, nil)
	}
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading ai trace: %w", err))
		return
	}
	if aiTrace == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "AI trace not found"})
		return
	}

	response := AiTraceDetailResponse{
		AiTrace: aiTrace,
	}

	if aiTrace.StorageKey != "" {
		data, err := storage.Store.Read(c, aiTrace.StorageKey)
		if err == nil {
			response.Conversation = json.RawMessage(data)
		} else {
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to read AI trace conversation (key=%s): %w", aiTrace.StorageKey, err))
		}
	}

	c.JSON(http.StatusOK, response)
}

var AiTraceController = aiTraceController{}
