package controllers

import (
	"backend/app/controllers/clientcontrollers"
	"backend/app/middleware"
	"os"

	"github.com/gin-gonic/gin"
)

var ExtensionRoutes []func(router *gin.RouterGroup)

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
	router.OPTIONS("/report", middleware.CORSReport)
	router.POST("/report", middleware.CORSReport, middleware.UseClientAuth, middleware.UseGzip, clientcontrollers.ClientController.Report)

	// Project management
	router.GET("/projects", middleware.UseAppAuth, ProjectController.ListProjects)
	router.POST("/projects", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ProjectController.CreateProject)

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
	router.GET("/endpoints/slow", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointController.GetSlowEndpoint)
	router.POST("/endpoints/slow", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, EndpointController.SetSlowEndpoint)
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
	router.POST("/login", middleware.Transactional, AuthController.Login)
	router.POST("/register", middleware.Transactional, AuthController.Register)

	if os.Getenv("CLOUD_MODE") != "true" {
		router.GET("/has-organizations", middleware.Transactional, AuthController.HasOrganizations)
	}

	// Password reset
	router.POST("/forgot-password", middleware.Transactional, PasswordResetController.ForgotPassword)
	router.GET("/password-reset/:token", PasswordResetController.ValidateToken)
	router.POST("/password-reset/:token", middleware.Transactional, PasswordResetController.ResetPassword)

	// Organization settings (admin/owner access)
	router.GET("/organizations/:organizationId/settings", middleware.UseAppAuth, middleware.RequireAdminAccess, OrganizationController.GetSettings)
	router.PUT("/organizations/:organizationId/settings", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, OrganizationController.UpdateSettings)
	router.GET("/organizations/:organizationId/members", middleware.UseAppAuth, middleware.RequireAdminAccess, OrganizationController.GetMembers)

	// Member management (admin/owner) - TRANSACTIONAL
	router.PUT("/organizations/:organizationId/members/:userId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, MemberController.UpdateRole)
	router.DELETE("/organizations/:organizationId/members/:userId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, MemberController.RemoveMember)

	// Invitations management (admin/owner) - TRANSACTIONAL
	router.POST("/organizations/:organizationId/invitations", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, InvitationController.InviteUser)
	router.GET("/organizations/:organizationId/invitations", middleware.UseAppAuth, middleware.RequireAdminAccess, InvitationController.ListInvitations)
	router.DELETE("/organizations/:organizationId/invitations/:id", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, InvitationController.RevokeInvitation)

	// Public invitation endpoints - TRANSACTIONAL
	router.GET("/invitations/:token", InvitationController.GetInvitationInfo)
	router.POST("/invitations/:token/accept", middleware.Transactional, InvitationController.AcceptInvitation)
	router.POST("/invitations/:token/accept-existing", middleware.UseAppAuth, middleware.Transactional, InvitationController.AcceptExistingUser)

	// Source map management
	router.POST("/projects/source-map-token", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ProjectController.GenerateSourceMapToken)
	router.POST("/sourcemaps/upload", middleware.UseSourceMapAuth, SourceMapController.Upload)

	for _, register := range ExtensionRoutes {
		register(router)
	}
}
