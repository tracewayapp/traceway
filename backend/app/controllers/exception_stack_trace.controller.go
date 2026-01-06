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
	ProjectId  string           `json:"projectId"`
	FromDate   time.Time        `json:"fromDate"`
	ToDate     time.Time        `json:"toDate"`
	OrderBy    string           `json:"orderBy"`
	Pagination PaginationParams `json:"pagination"`
	Search     string           `json:"search"`
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

	exceptions, total, err := repositories.ExceptionStackTraceRepository.FindGrouped(c, request.ProjectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.Search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

var ExceptionStackTraceController = exceptionStackTraceController{}
