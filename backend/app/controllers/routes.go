package controllers

import (
	"backend/app/controllers/clientcontrollers"
	"backend/app/middleware"

	"github.com/gin-gonic/gin"
)

type PaginationParams struct {
	Page     int `json:"page" binding:"min=1"`
	PageSize int `json:"pageSize" binding:"min=1,max=100"`
}

type PaginatedResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

func RegisterControllers(router *gin.RouterGroup) {
	router.POST("/report", middleware.UseClientAuth, middleware.UseGzip, clientcontrollers.ClientController.Report)

	// Project management
	router.GET("/projects", middleware.UseAppAuth, ProjectController.ListProjects)
	router.POST("/projects", middleware.UseAppAuth, ProjectController.CreateProject)
	router.GET("/projects/:id", middleware.UseAppAuth, ProjectController.GetProject)

	router.POST("/stats", middleware.UseAppAuth, MetricRecordController.FindHomepageStats)
	router.GET("/dashboard", middleware.UseAppAuth, DashboardController.GetDashboard)
	router.GET("/dashboard/overview", middleware.UseAppAuth, DashboardController.GetDashboardOverview)

	// Metrics endpoints (split by category)
	router.GET("/metrics/application", middleware.UseAppAuth, MetricsController.GetApplicationMetrics)
	router.GET("/metrics/stats", middleware.UseAppAuth, MetricsController.GetStatsMetrics)
	router.GET("/metrics/server", middleware.UseAppAuth, MetricsController.GetServerMetrics)

	// Endpoints
	router.POST("/endpoints", middleware.UseAppAuth, EndpointController.FindAllEndpoints)
	router.POST("/endpoints/grouped", middleware.UseAppAuth, EndpointController.FindGroupedByEndpoint)
	router.POST("/endpoints/endpoint", middleware.UseAppAuth, EndpointController.FindByEndpoint)
	router.POST("/endpoints/:endpointId", middleware.UseAppAuth, EndpointDetailController.GetEndpointDetail)

	// Tasks
	router.POST("/tasks", middleware.UseAppAuth, TaskController.FindAllTasks)
	router.POST("/tasks/grouped", middleware.UseAppAuth, TaskController.FindGroupedByTaskName)
	router.POST("/tasks/task", middleware.UseAppAuth, TaskController.FindByTaskName)
	router.POST("/tasks/:taskId", middleware.UseAppAuth, TaskDetailController.GetTaskDetail)
	router.POST("/exception-stack-traces", middleware.UseAppAuth, ExceptionStackTraceController.FindGrouppedExceptionStackTraces)
	router.POST("/exception-stack-traces/archive", middleware.UseAppAuth, ExceptionStackTraceController.ArchiveExceptions)
	router.POST("/exception-stack-traces/unarchive", middleware.UseAppAuth, ExceptionStackTraceController.UnarchiveExceptions)
	router.POST("/exception-stack-traces/:hash", middleware.UseAppAuth, ExceptionStackTraceController.FindByHash)

	// Auth
	router.POST("/login", AuthController.Login)
}
