package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

type transactionController struct{}

type TransactionSearchRequest struct {
	ProjectId     string           `json:"projectId"`
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
}

type EndpointTransactionsRequest struct {
	ProjectId     string           `json:"projectId"`
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
}

func (e transactionController) FindAllTransactions(c *gin.Context) {
	var request TransactionSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	transactions, total, err := repositories.TransactionRepository.FindAll(c, request.ProjectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy)
	if err != nil {
		panic(err)
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.Transaction]{
		Data: transactions,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (e transactionController) FindGroupedByEndpoint(c *gin.Context) {
	var request TransactionSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stats, total, err := repositories.TransactionRepository.FindGroupedByEndpoint(c, request.ProjectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection)
	if err != nil {
		panic(err)
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.EndpointStats]{
		Data: stats,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (e transactionController) FindByEndpoint(c *gin.Context) {
	rawEndpoint := c.Query("endpoint")
	if rawEndpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint is required"})
		return
	}

	// URL decode the endpoint (it may contain encoded slashes and spaces)
	endpoint, err := url.PathUnescape(rawEndpoint)
	if err != nil {
		endpoint = rawEndpoint // fallback to raw value if decoding fails
	}

	var request EndpointTransactionsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	transactions, total, err := repositories.TransactionRepository.FindByEndpoint(c, request.ProjectId, endpoint, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection)
	if err != nil {
		panic(err)
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.Transaction]{
		Data: transactions,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

var TransactionController = transactionController{}
