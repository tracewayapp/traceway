package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/controllers/clientcontrollers"
	"github.com/tracewayapp/traceway/backend/app/controllers/otelcontrollers"
	"github.com/tracewayapp/traceway/backend/app/middleware"

	"github.com/gin-gonic/gin"
)

var ExtensionRoutes []func(router *gin.RouterGroup)

var postPreflight = []string{http.MethodPost, http.MethodOptions}

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

func paginationFromQuery(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "25"))
	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func buildPagination(page, pageSize, total int) Pagination {
	totalPages := int64(0)
	if total > 0 {
		totalPages = int64((total + pageSize - 1) / pageSize)
	}
	return Pagination{Page: page, PageSize: pageSize, Total: int64(total), TotalPages: totalPages}
}

func RegisterControllers(router *gin.RouterGroup) {
	router.OPTIONS("/report", middleware.CORSReport)
	router.POST("/report", middleware.CORSReport, middleware.UseClientAuth, middleware.IngestAdmission, middleware.UseGzip, clientcontrollers.ClientController.Report)

	router.OPTIONS("/profiles/ingest", middleware.CORSReport)
	router.POST("/profiles/ingest", middleware.CORSReport, middleware.UseClientAuth, middleware.IngestAdmission, middleware.UseGzip, clientcontrollers.ProfileIngestController.Ingest)

	otelGroup := router.Group("/otel")
	otelGroup.OPTIONS("/v1/traces", middleware.CORSReport)
	otelGroup.OPTIONS("/v1/metrics", middleware.CORSReport)
	otelGroup.OPTIONS("/v1/logs", middleware.CORSReport)
	otelGroup.OPTIONS("/v1development/profiles", middleware.CORSReport)
	otelGroup.POST("/v1/traces", middleware.CORSReport, middleware.UseClientAuth, middleware.IngestAdmission, otelcontrollers.OtelController.ExportTraces)
	otelGroup.POST("/v1/metrics", middleware.CORSReport, middleware.UseClientAuth, middleware.IngestAdmission, otelcontrollers.OtelController.ExportMetrics)
	otelGroup.POST("/v1/logs", middleware.CORSReport, middleware.UseClientAuth, middleware.IngestAdmission, otelcontrollers.OtelController.ExportLogs)
	otelGroup.POST("/v1development/profiles", middleware.CORSReport, middleware.UseClientAuth, middleware.IngestAdmission, otelcontrollers.OtelController.ExportProfiles)

	router.GET("/projects", middleware.UseAppAuth, ProjectController.ListProjects)
	router.POST("/projects", middleware.UseAppAuth, middleware.RequireProjectAccess, ProjectController.CreateProject)
	router.POST("/projects/batch", middleware.UseAppAuth, middleware.Transactional, ProjectController.BatchCreateProjects)
	router.PUT("/projects", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ProjectController.UpdateProject)
	router.DELETE("/projects", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, ProjectController.DeleteProject)

	router.GET("/health/deep", middleware.UseHealthDeepToken, HealthDeepController.Get)

	router.POST("/stats", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricRecordController.FindHomepageStats)
	router.GET("/dashboard", middleware.UseAppAuth, middleware.RequireProjectAccess, DashboardController.GetDashboard)
	router.GET("/dashboard/overview", middleware.UseAppAuth, middleware.RequireProjectAccess, DashboardController.GetDashboardOverview)

	router.GET("/metrics/application", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricsController.GetApplicationMetrics)
	router.GET("/metrics/stats", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricsController.GetStatsMetrics)
	router.GET("/metrics/server", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricsController.GetServerMetrics)

	router.POST("/metrics/query", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricQueryController.Query)
	router.GET("/metrics/discover", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricQueryController.Discover)
	router.GET("/metrics/discover/tags", middleware.UseAppAuth, middleware.RequireProjectAccess, MetricQueryController.DiscoverTags)
	router.GET("/metrics/discover/org", middleware.UseAppAuth, MetricQueryController.DiscoverOrg)
	router.PUT("/metrics/registry", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, MetricQueryController.UpdateRegistry)

	router.GET("/dashboards", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, DashboardsController.List)
	router.POST("/dashboards", middleware.UseAppAuth, middleware.Transactional, DashboardsController.Create)
	router.GET("/dashboards/library", middleware.UseAppAuth, middleware.Transactional, DashboardsController.Library)
	router.POST("/dashboards/populate-defaults", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, DashboardsController.PopulateDefaults)
	router.GET("/dashboards/starred", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, DashboardsController.ListStarred)
	router.PUT("/dashboards/reorder", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, DashboardsController.Reorder)
	router.GET("/dashboards/:id", middleware.UseAppAuth, middleware.Transactional, DashboardsController.Get)
	router.PUT("/dashboards/:id", middleware.UseAppAuth, middleware.Transactional, DashboardsController.Update)
	router.DELETE("/dashboards/:id", middleware.UseAppAuth, middleware.Transactional, DashboardsController.Delete)
	router.GET("/dashboards/export", middleware.UseAppAuth, middleware.Transactional, DashboardsController.ExportOrganization)
	router.POST("/dashboards/import", middleware.UseAppAuth, middleware.Transactional, DashboardsController.Import)
	router.POST("/dashboards/import/grafana", middleware.UseAppAuth, middleware.Transactional, DashboardsController.ImportGrafana)
	router.GET("/dashboards/:id/export", middleware.UseAppAuth, middleware.Transactional, DashboardsController.Export)
	router.PUT("/dashboards/:id/apply", middleware.UseAppAuth, middleware.Transactional, DashboardsController.Apply)
	router.DELETE("/dashboards/:id/apply/:projectId", middleware.UseAppAuth, middleware.Transactional, DashboardsController.Unapply)
	router.POST("/dashboards/:id/copy", middleware.UseAppAuth, middleware.Transactional, DashboardsController.Copy)

	router.POST("/dashboards/:id/widgets", middleware.UseAppAuth, middleware.Transactional, DashboardsController.AddWidget)
	router.PUT("/dashboards/:id/widgets/reorder", middleware.UseAppAuth, middleware.Transactional, DashboardsController.ReorderWidgets)
	router.PUT("/dashboards/:id/widgets/:wid", middleware.UseAppAuth, middleware.Transactional, DashboardsController.UpdateWidget)
	router.DELETE("/dashboards/:id/widgets/:wid", middleware.UseAppAuth, middleware.Transactional, DashboardsController.DeleteWidget)
	router.PUT("/dashboards/:id/widgets/:wid/star", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, DashboardsController.ToggleStar)

	router.PUT("/starred-widgets/reorder", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, DashboardsController.ReorderStarred)
	router.PUT("/starred-widgets/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, DashboardsController.UpdateStarredLayout)

	router.GET("/dashboard-templates", middleware.UseAppAuth, middleware.Transactional, DashboardTemplateController.List)
	router.POST("/dashboard-templates/:key/install", middleware.UseAppAuth, middleware.Transactional, DashboardTemplateController.Install)

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

	router.POST("/profiles/grouped", middleware.UseAppAuth, middleware.RequireProjectAccess, ProfileController.FindGroupedByService)
	router.POST("/profiles/series", middleware.UseAppAuth, middleware.RequireProjectAccess, ProfileController.GetSeries)
	router.POST("/profiles/series/breakdown", middleware.UseAppAuth, middleware.RequireProjectAccess, ProfileController.GetSeriesBreakdown)
	router.POST("/profiles/dimensions", middleware.UseAppAuth, middleware.RequireProjectAccess, ProfileController.DiscoverDimensions)
	router.POST("/profiles/types", middleware.UseAppAuth, middleware.RequireProjectAccess, ProfileController.DiscoverTypes)
	router.POST("/profiles/flamegraph", middleware.UseAppAuth, middleware.RequireProjectAccess, ProfileController.GetFlameGraph)
	router.POST("/profiles/top", middleware.UseAppAuth, middleware.RequireProjectAccess, ProfileController.GetTopFunctions)
	router.POST("/profiles/pprof", middleware.UseAppAuth, middleware.RequireProjectAccess, ProfileController.DownloadPprof)
	router.POST("/profiles/labels", middleware.UseAppAuth, middleware.RequireProjectAccess, ProfileController.DiscoverLabels)

	router.POST("/ai-traces/grouped", middleware.UseAppAuth, middleware.RequireProjectAccess, AiTraceController.FindGroupedByTraceName)
	router.POST("/ai-traces/trace", middleware.UseAppAuth, middleware.RequireProjectAccess, AiTraceController.FindByTraceName)
	router.POST("/ai-traces/:traceId", middleware.UseAppAuth, middleware.RequireProjectAccess, AiTraceController.GetAiTraceDetail)

	router.POST("/ai-conversations/grouped", middleware.UseAppAuth, middleware.RequireProjectAccess, AiTraceController.FindConversations)
	router.POST("/ai-conversations/conversation", middleware.UseAppAuth, middleware.RequireProjectAccess, AiTraceController.GetConversationDetail)
	router.POST("/ai-users/grouped", middleware.UseAppAuth, middleware.RequireProjectAccess, AiTraceController.FindAiUsers)

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

	router.Match(postPreflight, "/auth/device/authorize", middleware.OAuthCors, middleware.RateLimitPerIP(10, time.Minute), DeviceAuthController.Authorize)
	router.Match(postPreflight, "/auth/device/token", middleware.OAuthCors, DeviceAuthController.Token)
	router.Match(postPreflight, "/auth/token", middleware.OAuthCors, DeviceAuthController.Token)
	router.Match(postPreflight, "/auth/logout", middleware.OAuthCors, DeviceAuthController.Logout)
	router.GET("/device", middleware.UseAppAuth, DeviceAuthController.Lookup)
	router.POST("/device/approve", middleware.UseAppAuth, middleware.Transactional, DeviceAuthController.Approve)
	router.POST("/device/deny", middleware.UseAppAuth, middleware.Transactional, DeviceAuthController.Deny)

	router.Match(postPreflight, "/oauth/register", middleware.OAuthCors, middleware.RateLimitPerIP(10, time.Minute), middleware.Transactional, OauthAuthorizeController.Register)
	router.GET("/oauth/client", middleware.UseAppAuth, OauthAuthorizeController.Lookup)
	router.POST("/oauth/approve", middleware.UseAppAuth, middleware.Transactional, OauthAuthorizeController.Approve)
	router.POST("/oauth/deny", middleware.UseAppAuth, OauthAuthorizeController.Deny)

	router.POST("/personal-access-tokens", middleware.UseAppAuth, middleware.Transactional, PATController.Create)
	router.GET("/personal-access-tokens", middleware.UseAppAuth, PATController.List)
	router.DELETE("/personal-access-tokens/:id", middleware.UseAppAuth, middleware.Transactional, PATController.Revoke)

	router.POST("/setup-tokens", middleware.UseAppAuth, middleware.RateLimitPerUser(10, time.Hour), middleware.Transactional, SetupTokenController.Create)
	router.GET("/setup/session", middleware.RateLimitPerIP(60, time.Minute), middleware.UseSetupAuth, middleware.Transactional, SetupController.GetSession)
	router.PUT("/setup/plan", middleware.RateLimitPerIP(20, time.Minute), middleware.UseSetupAuth, middleware.Transactional, SetupController.SubmitPlan)
	router.GET("/setup/plan", middleware.RateLimitPerIP(60, time.Minute), middleware.UseSetupAuth, middleware.Transactional, SetupController.GetPlan)
	router.GET("/setup/drafts", middleware.UseAppAuth, middleware.RateLimitPerUser(60, time.Minute), middleware.Transactional, SetupController.ListDraft)
	router.POST("/setup/drafts/:id/approve", middleware.UseAppAuth, middleware.Transactional, SetupController.ApproveDraft)
	router.POST("/setup/drafts/:id/reject", middleware.UseAppAuth, middleware.Transactional, SetupController.RejectDraft)

	// The OAuth discovery documents (/.well-known/oauth-*) are registered on the
	// root engine in cmd/run.go, not here: RFC 8414 / RFC 9728 clients fetch them
	// at the origin root, not under /api.

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
	router.GET("/organizations/:organizationId/members/:userId/project-roles", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, MemberController.GetProjectRoles)
	router.PUT("/organizations/:organizationId/members/:userId/project-roles/:projectId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, MemberController.UpdateProjectRole)

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

	router.GET("/synthetics/checks", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, SyntheticCheckController.List)
	router.POST("/synthetics/overview", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, SyntheticCheckController.Overview)
	router.POST("/synthetics/checks", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, SyntheticCheckController.Create)
	router.GET("/synthetics/checks/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, SyntheticCheckController.Get)
	router.PUT("/synthetics/checks/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, SyntheticCheckController.Update)
	router.DELETE("/synthetics/checks/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, SyntheticCheckController.Delete)
	router.POST("/synthetics/checks/:id/run", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, SyntheticCheckController.RunNow)
	router.POST("/synthetics/checks/:id/results", middleware.UseAppAuth, middleware.RequireProjectAccess, SyntheticCheckController.Results)
	router.GET("/synthetics/screenshot", middleware.UseAppAuth, middleware.RequireProjectAccess, SyntheticCheckController.Screenshot)
	router.GET("/synthetics/output", middleware.UseAppAuth, middleware.RequireProjectAccess, SyntheticCheckController.Output)
	router.POST("/synthetics/checks/:id/series", middleware.UseAppAuth, middleware.RequireProjectAccess, SyntheticCheckController.Series)
	router.GET("/synthetics/open-count", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, SyntheticCheckController.OpenCount)

	// No Transactional on either runner route: Poll holds the request for up
	// to 25s (a held transaction would starve the single-connection SQLite
	// main DB) and Result finalizes through ProcessOutcome, which manages its
	// own transactions.
	router.POST("/runners/poll", middleware.UseRunnerAuth, SyntheticRunnerController.Poll)
	router.POST("/runners/results/:runId", middleware.UseRunnerAuth, SyntheticRunnerController.Result)

	router.GET("/organizations/:organizationId/status-pages", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, StatusPageController.List)
	router.POST("/organizations/:organizationId/status-pages", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, StatusPageController.Create)
	router.PUT("/organizations/:organizationId/status-pages/:id", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, StatusPageController.Update)
	router.DELETE("/organizations/:organizationId/status-pages/:id", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, StatusPageController.Delete)
	router.POST("/organizations/:organizationId/status-pages/:id/logo", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, StatusPageController.UploadLogo)

	router.GET("/organizations/:organizationId/incidents", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, IncidentController.List)
	router.GET("/organizations/:organizationId/incidents/:incidentId/updates", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, IncidentController.ListUpdates)
	router.GET("/organizations/:organizationId/status-pages/:id/incidents", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, IncidentController.ListForStatusPage)
	router.POST("/organizations/:organizationId/status-pages/:id/incidents", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, IncidentController.Create)
	router.PUT("/organizations/:organizationId/incidents/:incidentId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, IncidentController.Update)
	router.DELETE("/organizations/:organizationId/incidents/:incidentId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, IncidentController.Delete)
	router.POST("/organizations/:organizationId/incidents/:incidentId/updates", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, IncidentController.CreateUpdate)
	router.DELETE("/organizations/:organizationId/incidents/:incidentId/updates/:updateId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, IncidentController.DeleteUpdate)

	router.GET("/post-mortems", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, PostMortemController.List)
	router.GET("/post-mortems/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, PostMortemController.Get)
	router.GET("/post-mortems/:id/activity", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, PostMortemController.Activity)
	router.POST("/post-mortems", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, PostMortemController.Create)
	router.PUT("/post-mortems/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, PostMortemController.Update)
	router.DELETE("/post-mortems/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.RequireWriteAccess, middleware.Transactional, PostMortemController.Delete)
	// Anonymous, rate-limited, cached ~30s per slug; no Transactional (it
	// reads main DB + telemetry through its own short transactions).
	router.GET("/status/:slug", middleware.RateLimitPerIP(30, time.Minute), StatusPageController.Public)
	router.GET("/status/:slug/logo", middleware.RateLimitPerIP(60, time.Minute), StatusPageController.PublicLogo)
	// Maps a CNAMEd vanity host to its status page slug (called by the SPA
	// when an anonymous visitor lands on / of an unrecognized host).
	router.GET("/status-domains/resolve", middleware.RateLimitPerIP(60, time.Minute), StatusPageController.ResolveHost)

	router.GET("/organizations/:organizationId/teams", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, TeamController.List)
	router.POST("/organizations/:organizationId/teams", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, TeamController.Create)
	router.PUT("/organizations/:organizationId/teams/:teamId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, TeamController.Update)
	router.DELETE("/organizations/:organizationId/teams/:teamId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, TeamController.Delete)
	router.PUT("/organizations/:organizationId/teams/:teamId/members", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, TeamController.SetMembers)
	router.PUT("/organizations/:organizationId/teams/:teamId/projects", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, TeamController.SetProjects)

	router.GET("/organizations/:organizationId/schedules", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, OncallController.ListSchedules)
	router.POST("/organizations/:organizationId/schedules", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, OncallController.CreateSchedule)
	router.GET("/organizations/:organizationId/schedules/:scheduleId", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, OncallController.GetSchedule)
	router.PUT("/organizations/:organizationId/schedules/:scheduleId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, OncallController.UpdateSchedule)
	router.DELETE("/organizations/:organizationId/schedules/:scheduleId", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, OncallController.DeleteSchedule)
	router.GET("/organizations/:organizationId/schedules/:scheduleId/timeline", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, OncallController.Timeline)
	router.POST("/organizations/:organizationId/schedules/:scheduleId/overrides", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, OncallController.CreateOverride)
	router.DELETE("/organizations/:organizationId/schedules/:scheduleId/overrides/:overrideId", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, OncallController.DeleteOverride)
	router.GET("/organizations/:organizationId/oncall/now", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, OncallController.Now)
	router.GET("/oncall/current", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, OncallController.Current)

	router.GET("/escalation-policies", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, EscalationPolicyController.ListForProject)
	router.GET("/organizations/:organizationId/escalation-policies", middleware.UseAppAuth, middleware.RequireOrganizationAccess, middleware.Transactional, EscalationPolicyController.ListForOrganization)
	router.POST("/organizations/:organizationId/escalation-policies", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, EscalationPolicyController.Create)
	router.PUT("/organizations/:organizationId/escalation-policies/:id", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, EscalationPolicyController.Update)
	router.DELETE("/organizations/:organizationId/escalation-policies/:id", middleware.UseAppAuth, middleware.RequireAdminAccess, middleware.Transactional, EscalationPolicyController.Delete)

	router.POST("/pages", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, PageController.List)
	router.POST("/pages/for-issues", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, PageController.UnresolvedForIssues)
	router.POST("/pages/bulk-acknowledge", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, PageController.BulkAcknowledge)
	router.POST("/pages/bulk-resolve", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, PageController.BulkResolve)
	router.GET("/pages/open-count", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, PageController.OpenCount)
	router.GET("/pages/:id", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, PageController.Get)
	router.POST("/pages/:id/acknowledge", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, PageController.Acknowledge)
	router.POST("/pages/:id/resolve", middleware.UseAppAuth, middleware.RequireProjectAccess, middleware.Transactional, PageController.Resolve)

	router.GET("/contact-methods", middleware.UseAppAuth, middleware.Transactional, ContactMethodController.List)
	router.POST("/contact-methods", middleware.UseAppAuth, middleware.RateLimitPerUser(20, 10*time.Minute), middleware.Transactional, ContactMethodController.Create)
	router.PUT("/contact-methods/:id", middleware.UseAppAuth, middleware.RateLimitPerUser(30, 10*time.Minute), middleware.Transactional, ContactMethodController.Update)
	router.DELETE("/contact-methods/:id", middleware.UseAppAuth, middleware.Transactional, ContactMethodController.Delete)
	// Rate limited like the mutating routes: a contact method can point at any
	// address/number/webhook, so an uncapped test button is an outbound relay.
	router.POST("/contact-methods/:id/test", middleware.UseAppAuth, middleware.RateLimitPerUser(10, 10*time.Minute), ContactMethodController.Test)
	// No Transactional: Verify answers 422 for a wrong code, and the consumed
	// attempt must still commit, so it manages its own transactions (the same
	// reason the OAuth grant endpoints do).
	router.POST("/contact-methods/:id/verify", middleware.UseAppAuth, middleware.RateLimitPerUser(10, time.Minute), ContactMethodController.Verify)
	router.POST("/contact-methods/:id/resend-code", middleware.UseAppAuth, middleware.RateLimitPerUser(3, 5*time.Minute), middleware.Transactional, ContactMethodController.ResendCode)

	router.GET("/user-notification-rules", middleware.UseAppAuth, middleware.Transactional, UserNotificationRuleController.Get)
	router.PUT("/user-notification-rules", middleware.UseAppAuth, middleware.Transactional, UserNotificationRuleController.Put)

	router.GET("/ack/:token", middleware.RateLimitPerIP(30, time.Minute), middleware.Transactional, AckController.Get)
	router.POST("/ack/:token", middleware.RateLimitPerIP(10, time.Minute), middleware.Transactional, AckController.Acknowledge)

	for _, register := range ExtensionRoutes {
		register(router)
	}
}
