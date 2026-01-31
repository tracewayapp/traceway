package controllers

import (
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type exceptionStackTraceController struct{}

type ExceptionSearchRequest struct {
	FromDate        time.Time        `json:"fromDate"`
	ToDate          time.Time        `json:"toDate"`
	OrderBy         string           `json:"orderBy"`
	Pagination      PaginationParams `json:"pagination"`
	Search          string           `json:"search"`
	SearchType      string           `json:"searchType"`
	IncludeArchived bool             `json:"includeArchived"`
}

type ArchiveRequest struct {
	Hashes []string `json:"hashes"`
}

type ExceptionDetailRequest struct {
	Pagination PaginationParams `json:"pagination"`
}

type ExceptionDetailResponse struct {
	Group       *models.ExceptionGroup       `json:"group"`
	Occurrences []models.ExceptionStackTrace `json:"occurrences"`
	Pagination  Pagination                   `json:"pagination"`
}

func (e exceptionStackTraceController) FindGrouppedExceptionStackTraces(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ExceptionSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	span := traceway.StartSpan(c, "loading grouped exceptions")
	exceptions, total, err := repositories.ExceptionStackTraceRepository.FindGrouped(c, projectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.Search, request.SearchType, request.IncludeArchived)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading exceptions: %w", err))
		return
	}

	// Get hourly trends for all returned exception hashes (last 24 hours)
	if len(exceptions) > 0 {
		hashes := make([]string, len(exceptions))
		for i, ex := range exceptions {
			hashes[i] = ex.ExceptionHash
		}

		// Calculate 24-hour window
		now := time.Now()
		start24h := now.Add(-24 * time.Hour)

		span = traceway.StartSpan(c, "loading hourly trends")
		trends, err := repositories.ExceptionStackTraceRepository.GetHourlyTrendForHashes(c, projectId, hashes, start24h, now)
		span.End()
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading trends: %w", err))
			return
		}

		// Attach trends to each exception group
		for i := range exceptions {
			if trend, ok := trends[exceptions[i].ExceptionHash]; ok {
				exceptions[i].HourlyTrend = trend
			} else {
				exceptions[i].HourlyTrend = []models.ExceptionTrendPoint{}
			}
		}
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.ExceptionGroup]{
		Data: exceptions,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (e exceptionStackTraceController) FindByHash(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	exceptionHash := c.Param("hash")
	if exceptionHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exception hash is required"})
		return
	}

	var request ExceptionDetailRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		// Default pagination if not provided
		request.Pagination = PaginationParams{Page: 1, PageSize: 20}
	}

	span := traceway.StartSpan(c, "loading exception by hash")
	group, occurrences, total, err := repositories.ExceptionStackTraceRepository.FindByHash(c, projectId, exceptionHash, request.Pagination.Page, request.Pagination.PageSize)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading the group: %w", err))
		return
	}

	if group == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Exception not found"})
		return
	}

	c.JSON(http.StatusOK, ExceptionDetailResponse{
		Group:       group,
		Occurrences: occurrences,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (e exceptionStackTraceController) ArchiveExceptions(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ArchiveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(request.Hashes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hashes array is required"})
		return
	}

	err = repositories.ExceptionStackTraceRepository.ArchiveByHashes(c, projectId, request.Hashes)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error archiving %s: %w", strings.Join(request.Hashes, ","), err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"archived": len(request.Hashes)})
}

func (e exceptionStackTraceController) UnarchiveExceptions(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request ArchiveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(request.Hashes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hashes array is required"})
		return
	}

	err = repositories.ExceptionStackTraceRepository.UnarchiveByHashes(c, projectId, request.Hashes)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error unarchiving %s: %w", strings.Join(request.Hashes, ","), err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"unarchived": len(request.Hashes)})
}

func (e exceptionStackTraceController) FindById(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	exceptionId, err := uuid.Parse(c.Param("exceptionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid exception id"})
		return
	}

	span := traceway.StartSpan(c, "loading exception by id")
	exception, err := repositories.ExceptionStackTraceRepository.FindById(c, projectId, exceptionId)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading the exception: %w", err))
		return
	}

	if exception == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Exception not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"exception": exception})
}

var ExceptionStackTraceController = exceptionStackTraceController{}
