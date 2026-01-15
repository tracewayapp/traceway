package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

type endpointController struct{}

type EndpointSearchRequest struct {
	ProjectId     string           `json:"projectId"`
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
}

type EndpointInstancesRequest struct {
	ProjectId     string           `json:"projectId"`
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
}

type EndpointInstancesResponse struct {
	Data       []models.Endpoint           `json:"data"`
	Stats      *models.EndpointDetailStats `json:"stats"`
	Pagination Pagination                  `json:"pagination"`
}

func (e endpointController) FindAllEndpoints(c *gin.Context) {
	var request EndpointSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	endpoints, total, err := repositories.EndpointRepository.FindAll(c, request.ProjectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy)
	if err != nil {
		panic(err)
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.Endpoint]{
		Data: endpoints,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (e endpointController) FindGroupedByEndpoint(c *gin.Context) {
	var request EndpointSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stats, total, err := repositories.EndpointRepository.FindGroupedByEndpoint(c, request.ProjectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection)
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

func (e endpointController) FindByEndpoint(c *gin.Context) {
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

	var request EndpointInstancesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	endpoints, total, err := repositories.EndpointRepository.FindByEndpoint(c, request.ProjectId, endpoint, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection)
	if err != nil {
		panic(err)
	}

	// Get aggregate stats for this endpoint
	stats, err := repositories.EndpointRepository.GetEndpointStats(c, request.ProjectId, endpoint, request.FromDate, request.ToDate)
	if err != nil {
		// Don't fail the request if stats fail, just return nil stats
		stats = nil
	}

	c.JSON(http.StatusOK, EndpointInstancesResponse{
		Data:  endpoints,
		Stats: stats,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

var EndpointController = endpointController{}
