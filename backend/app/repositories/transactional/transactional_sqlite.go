//go:build !transactional_pg

package transactional

import sqliterepo "github.com/tracewayapp/traceway/backend/app/repositories/transactional/sqlite"

var (
	AgentAttemptEventRepository      = sqliterepo.AgentAttemptEventRepository
	AgentAttemptRepository           = sqliterepo.AgentAttemptRepository
	AgentLinkRepository              = sqliterepo.AgentLinkRepository
	AgentMessageRepository           = sqliterepo.AgentMessageRepository
	AgentProfileRepository           = sqliterepo.AgentProfileRepository
	AgentRunnerRepository            = sqliterepo.AgentRunnerRepository
	AuthorizationCodeRepository      = sqliterepo.AuthorizationCodeRepository
	CheckIncidentRepository          = sqliterepo.CheckIncidentRepository
	CheckRunRepository               = sqliterepo.CheckRunRepository
	DashboardRepository              = sqliterepo.DashboardRepository
	DashboardTemplateRepository      = sqliterepo.DashboardTemplateRepository
	DeviceAuthorizationRepository    = sqliterepo.DeviceAuthorizationRepository
	EscalationPolicyRepository       = sqliterepo.EscalationPolicyRepository
	IdentityRepository               = sqliterepo.IdentityRepository
	IncidentUpdateRepository         = sqliterepo.IncidentUpdateRepository
	IntegrationRepository            = sqliterepo.IntegrationRepository
	InvitationRepository             = sqliterepo.InvitationRepository
	MetricRegistryRepository         = sqliterepo.MetricRegistryRepository
	NotificationChannelRepository    = sqliterepo.NotificationChannelRepository
	NotificationRuleRepository       = sqliterepo.NotificationRuleRepository
	OncallOverrideRepository         = sqliterepo.OncallOverrideRepository
	OncallScheduleRepository         = sqliterepo.OncallScheduleRepository
	OAuthSessionRepository           = sqliterepo.OAuthSessionRepository
	OauthClientRepository            = sqliterepo.OauthClientRepository
	OrganizationRepository           = sqliterepo.OrganizationRepository
	OutboxRepository                 = sqliterepo.OutboxRepository
	PageNotificationRepository       = sqliterepo.PageNotificationRepository
	PageRepository                   = sqliterepo.PageRepository
	PersonalAccessTokenRepository    = sqliterepo.PersonalAccessTokenRepository
	PostMortemRepository             = sqliterepo.PostMortemRepository
	ProjectRepository                = sqliterepo.ProjectRepository
	ProjectTelemetrySourceRepository = sqliterepo.ProjectTelemetrySourceRepository
	ProjectUserRoleRepository        = sqliterepo.ProjectUserRoleRepository
	RefreshTokenRepository           = sqliterepo.RefreshTokenRepository
	RepositoryRepository             = sqliterepo.RepositoryRepository
	SetupPlanRepository              = sqliterepo.SetupPlanRepository
	SetupTokenRepository             = sqliterepo.SetupTokenRepository
	StatusPageRepository             = sqliterepo.StatusPageRepository
	SyntheticCheckRepository         = sqliterepo.SyntheticCheckRepository
	SyntheticRunnerRepository        = sqliterepo.SyntheticRunnerRepository
	TeamRepository                   = sqliterepo.TeamRepository
	UserContactMethodRepository      = sqliterepo.UserContactMethodRepository
	UserNotificationRuleRepository   = sqliterepo.UserNotificationRuleRepository
	UserRepository                   = sqliterepo.UserRepository
)
