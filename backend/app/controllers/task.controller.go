package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

type taskController struct{}

type TaskSearchRequest struct {
	ProjectId     string           `json:"projectId"`
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
}

type TaskInstancesRequest struct {
	ProjectId     string           `json:"projectId"`
	FromDate      time.Time        `json:"fromDate"`
	ToDate        time.Time        `json:"toDate"`
	OrderBy       string           `json:"orderBy"`
	SortDirection string           `json:"sortDirection"`
	Pagination    PaginationParams `json:"pagination"`
}

type TaskInstancesResponse struct {
	Data       []models.Task           `json:"data"`
	Stats      *models.TaskDetailStats `json:"stats"`
	Pagination Pagination              `json:"pagination"`
}

func (e taskController) FindAllTasks(c *gin.Context) {
	var request TaskSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tasks, total, err := repositories.TaskRepository.FindAll(c, request.ProjectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy)
	if err != nil {
		panic(err)
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.Task]{
		Data: tasks,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (e taskController) FindGroupedByTaskName(c *gin.Context) {
	var request TaskSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stats, total, err := repositories.TaskRepository.FindGroupedByTaskName(c, request.ProjectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection)
	if err != nil {
		panic(err)
	}

	c.JSON(http.StatusOK, PaginatedResponse[models.TaskStats]{
		Data: stats,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

func (e taskController) FindByTaskName(c *gin.Context) {
	rawTaskName := c.Query("task")
	if rawTaskName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task is required"})
		return
	}

	// URL decode the task name
	taskName, err := url.PathUnescape(rawTaskName)
	if err != nil {
		taskName = rawTaskName // fallback to raw value if decoding fails
	}

	var request TaskInstancesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tasks, total, err := repositories.TaskRepository.FindByTaskName(c, request.ProjectId, taskName, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection)
	if err != nil {
		panic(err)
	}

	// Get aggregate stats for this task
	stats, err := repositories.TaskRepository.GetTaskStats(c, request.ProjectId, taskName, request.FromDate, request.ToDate)
	if err != nil {
		// Don't fail the request if stats fail, just return nil stats
		stats = nil
	}

	c.JSON(http.StatusOK, TaskInstancesResponse{
		Data:  tasks,
		Stats: stats,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

var TaskController = taskController{}
