package controllers

import (
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/controllers/clientcontrollers"
	"github.com/tracewayapp/traceway/backend/app/controllers/otelcontrollers"
	"github.com/tracewayapp/traceway/backend/app/middleware"

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

	otelGroup := router.Group("/otel")
	otelGroup.OPTIONS("/v1/traces", middleware.CORSReport)
	otelGroup.OPTIONS("/v1/metrics", middleware.CORSReport)
	otelGroup.OPTIONS("/v1/logs", middleware.CORSReport)
	otelGroup.POST("/v1/traces", middleware.CORSReport, middleware.UseClientAuth, otelcontrollers.OtelController.ExportTraces)
	otelGroup.POST("/v1/metrics", middleware.CORSReport, middleware.UseClientAuth, otelcontrollers.OtelController.ExportMetrics)
	otelGroup.POST("/v1/logs", middleware.CORSReport, middleware.UseClientAuth, otelcontrollers.OtelController.ExportLogs)

	router.GET("/projects", middleware.UseAppAuth, ProjectController.ListProjects)
	router.POST("/projects", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ProjectController.CreateProject)
	router.PUT("/projects", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ProjectController.UpdateProject)
	router.DELETE("/projects", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ProjectController.DeleteProject)

	router.GET("/health/deep", middleware.UseAppAuth, HealthDeepController.Get)

	router.POST("/stats", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricRecordController.FindHomepageStats)
	router.GET("/dashboard", middleware.UseAppAuth, middleware.RequireProjectAccess, DashboardController.GetDashboard)
	router.GET("/dashboard/overview", middleware.UseAppAuth, middleware.RequireProjectAccess, DashboardController.GetDashboardOverview)

	router.GET("/metrics/application", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricsController.GetApplicationMetrics)
	router.GET("/metrics/stats", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricsController.GetStatsMetrics)
	router.GET("/metrics/server", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricsController.GetServerMetrics)

	router.POST("/metrics/query", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricQueryController.Query)
	router.GET("/metrics/discover", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricQueryController.Discover)
	router.GET("/metrics/discover/tags", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricQueryController.DiscoverTags)
	router.PUT("/metrics/registry", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, MetricQueryController.UpdateRegistry)

	router.GET("/widget-groups", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, WidgetGroupController.List)
	router.POST("/widget-groups", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, WidgetGroupController.Create)
	router.POST("/widget-groups/populate-defaults", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, WidgetGroupController.PopulateDefaults)
	router.GET("/widget-groups/starred", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, WidgetController.ListStarred)
	router.GET("/widget-groups/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, WidgetGroupController.GetWithWidgets)
	router.PUT("/widget-groups/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, WidgetGroupController.Update)
	router.DELETE("/widget-groups/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, WidgetGroupController.Delete)

	router.POST("/widget-groups/:id/widgets", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, WidgetController.Add)
	router.PUT("/widget-groups/:id/widgets/:wid", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, WidgetController.Update)
	router.PUT("/widget-groups/:id/widgets/:wid/move", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, WidgetController.Move)
	router.PUT("/widget-groups/:id/widgets/:wid/star", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, WidgetController.ToggleStar)
	router.DELETE("/widget-groups/:id/widgets/:wid", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, WidgetController.Delete)

	router.POST("/endpoints", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointController.FindAllEndpoints)
	router.POST("/endpoints/grouped", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointController.FindGroupedByEndpoint)
	router.POST("/endpoints/endpoint", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointController.FindByEndpoint)
	router.POST("/endpoints/chart", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointController.GetStackedChart)
	router.GET("/endpoints/slow", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointController.GetSlowEndpoint)
	router.POST("/endpoints/slow", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, EndpointController.SetSlowEndpoint)
	router.POST("/endpoints/:endpointId", middleware.UseAppAuth, middleware.RequireProjectAccess, EndpointDetailController.GetEndpointDetail)

	router.POST("/tasks", middleware.UseAppAuth, middleware.RequireProjectAccess, TaskController.FindAllTasks)
	router.POST("/tasks/grouped", middleware.UseAppAuth, middleware.RequireProjectAccess, TaskController.FindGroupedByTaskName)
	router.POST("/tasks/task", middleware.UseAppAuth, middleware.RequireProjectAccess, TaskController.FindByTaskName)
	router.POST("/tasks/:taskId", middleware.UseAppAuth, middleware.RequireProjectAccess, TaskDetailController.GetTaskDetail)

	router.POST("/sessions", middleware.UseAppAuth, middleware.RequireProjectAccess, SessionController.FindAllSessions)
	router.POST("/sessions/:sessionId", middleware.UseAppAuth, middleware.RequireProjectAccess, SessionDetailController.GetSessionDetail)
	router.GET("/sessions/:sessionId/recording", middleware.UseAppAuth, middleware.RequireProjectAccess, SessionDetailController.GetSessionRecording)

	router.POST("/ai-traces/grouped", middleware.UseAppAuth, middleware.RequireProjectAccess, AiTraceController.FindGroupedByTraceName)
	router.POST("/ai-traces/trace", middleware.UseAppAuth, middleware.RequireProjectAccess, AiTraceController.FindByTraceName)
	router.POST("/ai-traces/:traceId", middleware.UseAppAuth, middleware.RequireProjectAccess, AiTraceController.GetAiTraceDetail)

	router.POST("/distributed-traces/:distributedTraceId", middleware.UseAppAuth, DistributedTraceController.GetDistributedTrace)

	router.POST("/logs", middleware.UseAppAuth, middleware.RequireProjectAccess, LogController.List)

	router.POST("/exception-stack-traces", middleware.UseAppAuth, middleware.RequireProjectAccess, ExceptionStackTraceController.FindGrouppedExceptionStackTraces)
	router.POST("/exception-stack-traces/archive", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ExceptionStackTraceController.ArchiveExceptions)
	router.POST("/exception-stack-traces/unarchive", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ExceptionStackTraceController.UnarchiveExceptions)
	router.POST("/exception-stack-traces/by-id/:exceptionId", middleware.UseAppAuth, middleware.RequireProjectAccess, ExceptionStackTraceController.FindById)
	router.POST("/exception-stack-traces/:hash", middleware.UseAppAuth, middleware.RequireProjectAccess, ExceptionStackTraceController.FindByHash)

	router.POST("/login", middleware.Transactional, AuthController.Login)
	router.POST("/register", middleware.Transactional, AuthController.Register)
	router.GET("/me/login-bundle", middleware.UseAppAuth, middleware.Transactional, AuthController.LoginBundle)

	router.GET("/auth/providers", OAuthController.ListProviders)
	router.GET("/auth/start/:provider", middleware.Transactional, OAuthController.Begin)
	router.GET("/auth/callback/:provider", middleware.Transactional, OAuthController.Callback)
	router.POST("/auth/finish-setup", middleware.UseAppAuth, middleware.Transactional, OAuthController.FinishSetup)

	if config.Config.CloudMode != "true" {
		router.GET("/has-organizations", middleware.Transactional, AuthController.HasOrganizations)
	}

	router.POST("/forgot-password", middleware.Transactional, PasswordResetController.ForgotPassword)
	router.GET("/password-reset/:token", PasswordResetController.ValidateToken)
	router.POST("/password-reset/:token", middleware.Transactional, PasswordResetController.ResetPassword)

	router.GET("/organizations/:organizationId/settings", middleware.UseAppAuth, middleware.RequireAdminAccess, OrganizationController.GetSettings)
	router.PUT("/organizations/:organizationId/settings", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, OrganizationController.UpdateSettings)
	router.GET("/organizations/:organizationId/members", middleware.UseAppAuth, middleware.RequireAdminAccess, OrganizationController.GetMembers)

	router.PUT("/organizations/:organizationId/members/:userId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, MemberController.UpdateRole)
	router.DELETE("/organizations/:organizationId/members/:userId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, MemberController.RemoveMember)

	router.POST("/organizations/:organizationId/invitations", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, InvitationController.InviteUser)
	router.GET("/organizations/:organizationId/invitations", middleware.UseAppAuth, middleware.RequireAdminAccess, InvitationController.ListInvitations)
	router.DELETE("/organizations/:organizationId/invitations/:id", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, InvitationController.RevokeInvitation)

	router.GET("/invitations/:token", InvitationController.GetInvitationInfo)
	router.POST("/invitations/:token/accept", middleware.Transactional, InvitationController.AcceptInvitation)
	router.POST("/invitations/:token/accept-existing", middleware.UseAppAuth, middleware.Transactional, InvitationController.AcceptExistingUser)

	router.POST("/projects/source-map-token", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ProjectController.GenerateSourceMapToken)
	router.POST("/sourcemaps/upload", middleware.UseSourceMapAuth, SourceMapController.Upload)
	router.POST("/symbols/upload", middleware.UseSourceMapAuth, SymbolsController.Upload)

	router.GET("/notification-channels", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, NotificationChannelController.List)
	router.POST("/notification-channels", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, NotificationChannelController.Create)
	router.PUT("/notification-channels/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, NotificationChannelController.Update)
	router.DELETE("/notification-channels/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, NotificationChannelController.Delete)
	router.POST("/notification-channels/:id/test", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, NotificationChannelController.Test)

	router.GET("/notification-rules", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, NotificationRuleController.List)
	router.POST("/notification-rules", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, NotificationRuleController.Create)
	router.PUT("/notification-rules/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, NotificationRuleController.Update)
	router.DELETE("/notification-rules/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, NotificationRuleController.Delete)
	router.POST("/notification-rules/:id/toggle", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, NotificationRuleController.Toggle)
	router.POST("/notification-rules/:id/snooze", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, NotificationRuleController.Snooze)

	router.POST("/notification-history", middleware.UseAppAuth, middleware.RequireProjectAccess, NotificationHistoryController.List)

	for _, register := range ExtensionRoutes {
		register(router)
	}
}
