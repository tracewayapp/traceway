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
	router.POST("/projects", middleware.UseAppAuth, middleware.RequireWriteAccess, ProjectController.CreateProject)
	router.GET("/projects/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, ProjectController.GetProject)

	// Dashboard endpoints (projectId in query param)
	router.POST("/stats", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricRecordController.FindHomepageStats)
	router.GET("/dashboard", middleware.UseAppAuth, middleware.RequireProjectAccess, DashboardController.GetDashboard)
	router.GET("/dashboard/overview", middleware.UseAppAuth, middleware.RequireProjectAccess, DashboardController.GetDashboardOverview)

	// Metrics endpoints (projectId in query param)
	router.GET("/metrics/application", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricsController.GetApplicationMetrics)
	router.GET("/metrics/stats", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricsController.GetStatsMetrics)
	router.GET("/metrics/server", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricsController.GetServerMetrics)

	// Endpoints (projectId in body)
	router.POST("/endpoints", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointController.FindAllEndpoints)
	router.POST("/endpoints/grouped", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointController.FindGroupedByEndpoint)
	router.POST("/endpoints/endpoint", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointController.FindByEndpoint)
	router.POST("/endpoints/chart", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointController.GetStackedChart)
	router.POST("/endpoints/:endpointId", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointDetailController.GetEndpointDetail)

	// Tasks (projectId in body)
	router.POST("/tasks", middleware.UseAppAuth, middleware.RequireProjectAccess, TaskController.FindAllTasks)
	router.POST("/tasks/grouped", middleware.UseAppAuth, middleware.RequireProjectAccess, TaskController.FindGroupedByTaskName)
	router.POST("/tasks/task", middleware.UseAppAuth, middleware.RequireProjectAccess, TaskController.FindByTaskName)
	router.POST("/tasks/:taskId", middleware.UseAppAuth, middleware.RequireProjectAccess, TaskDetailController.GetTaskDetail)

	// Exceptions (projectId in body)
	router.POST("/exception-stack-traces", middleware.UseAppAuth, middleware.RequireProjectAccess, ExceptionStackTraceController.FindGrouppedExceptionStackTraces)
	router.POST("/exception-stack-traces/archive", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ExceptionStackTraceController.ArchiveExceptions)
	router.POST("/exception-stack-traces/unarchive", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ExceptionStackTraceController.UnarchiveExceptions)
	router.POST("/exception-stack-traces/by-id/:exceptionId", middleware.UseAppAuth, middleware.RequireProjectAccess, ExceptionStackTraceController.FindById)
	router.POST("/exception-stack-traces/:hash", middleware.UseAppAuth, middleware.RequireProjectAccess, ExceptionStackTraceController.FindByHash)

	// Auth
	router.POST("/login", AuthController.Login)
	router.POST("/register", AuthController.Register)
	router.GET("/me", middleware.UseAppAuth, AuthController.Me)
}
