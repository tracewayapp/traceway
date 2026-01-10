package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type exceptionStackTraceController struct{}

type ExceptionSearchRequest struct {
	ProjectId       string           `json:"projectId"`
	FromDate        time.Time        `json:"fromDate"`
	ToDate          time.Time        `json:"toDate"`
	OrderBy         string           `json:"orderBy"`
	Pagination      PaginationParams `json:"pagination"`
	Search          string           `json:"search"`
	IncludeArchived bool             `json:"includeArchived"`
}

type ArchiveRequest struct {
	ProjectId string   `json:"projectId"`
	Hashes    []string `json:"hashes"`
}

type ExceptionDetailRequest struct {
	ProjectId  string           `json:"projectId"`
	Pagination PaginationParams `json:"pagination"`
}

type ExceptionDetailResponse struct {
	Group       *models.ExceptionGroup       `json:"group"`
	Occurrences []models.ExceptionStackTrace `json:"occurrences"`
	Pagination  Pagination                   `json:"pagination"`
}

func (e exceptionStackTraceController) FindGrouppedExceptionStackTraces(c *gin.Context) {
	var request ExceptionSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	exceptions, total, err := repositories.ExceptionStackTraceRepository.FindGrouped(c, request.ProjectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.Search, request.IncludeArchived)
	if err != nil {
		panic(err)
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

		trends, err := repositories.ExceptionStackTraceRepository.GetHourlyTrendForHashes(c, request.ProjectId, hashes, start24h, now)
		if err != nil {
			panic(err)
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

	group, occurrences, total, err := repositories.ExceptionStackTraceRepository.FindByHash(c, request.ProjectId, exceptionHash, request.Pagination.Page, request.Pagination.PageSize)
	if err != nil {
		if errors.Is(err, repositories.ErrExceptionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Exception not found"})
			return
		}
		panic(err)
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
	var request ArchiveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(request.Hashes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hashes array is required"})
		return
	}

	err := repositories.ExceptionStackTraceRepository.ArchiveByHashes(c, request.ProjectId, request.Hashes)
	if err != nil {
		panic(err)
	}

	c.JSON(http.StatusOK, gin.H{"archived": len(request.Hashes)})
}

func (e exceptionStackTraceController) UnarchiveExceptions(c *gin.Context) {
	var request ArchiveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(request.Hashes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hashes array is required"})
		return
	}

	err := repositories.ExceptionStackTraceRepository.UnarchiveByHashes(c, request.ProjectId, request.Hashes)
	if err != nil {
		panic(err)
	}

	c.JSON(http.StatusOK, gin.H{"unarchived": len(request.Hashes)})
}

var ExceptionStackTraceController = exceptionStackTraceController{}
