//go:build transactional_pg

package transactional

import pgrepo "github.com/tracewayapp/traceway/backend/app/repositories/transactional/pg"

var (
	AgentAttemptEventRepository      = pgrepo.AgentAttemptEventRepository
	AgentAttemptRepository           = pgrepo.AgentAttemptRepository
	AgentLinkRepository              = pgrepo.AgentLinkRepository
	AgentMessageRepository           = pgrepo.AgentMessageRepository
	AgentProfileRepository           = pgrepo.AgentProfileRepository
	AgentRunnerRepository            = pgrepo.AgentRunnerRepository
	AuthorizationCodeRepository      = pgrepo.AuthorizationCodeRepository
	CheckIncidentRepository          = pgrepo.CheckIncidentRepository
	CheckRunRepository               = pgrepo.CheckRunRepository
	DashboardRepository              = pgrepo.DashboardRepository
	DashboardTemplateRepository      = pgrepo.DashboardTemplateRepository
	DeviceAuthorizationRepository    = pgrepo.DeviceAuthorizationRepository
	EscalationPolicyRepository       = pgrepo.EscalationPolicyRepository
	IdentityRepository               = pgrepo.IdentityRepository
	IncidentUpdateRepository         = pgrepo.IncidentUpdateRepository
	IntegrationRepository            = pgrepo.IntegrationRepository
	InvitationRepository             = pgrepo.InvitationRepository
	MetricRegistryRepository         = pgrepo.MetricRegistryRepository
	NotificationChannelRepository    = pgrepo.NotificationChannelRepository
	NotificationRuleRepository       = pgrepo.NotificationRuleRepository
	OncallOverrideRepository         = pgrepo.OncallOverrideRepository
	OncallScheduleRepository         = pgrepo.OncallScheduleRepository
	OAuthSessionRepository           = pgrepo.OAuthSessionRepository
	OauthClientRepository            = pgrepo.OauthClientRepository
	OrganizationRepository           = pgrepo.OrganizationRepository
	OutboxRepository                 = pgrepo.OutboxRepository
	PageNotificationRepository       = pgrepo.PageNotificationRepository
	PageRepository                   = pgrepo.PageRepository
	PersonalAccessTokenRepository    = pgrepo.PersonalAccessTokenRepository
	PostMortemRepository             = pgrepo.PostMortemRepository
	ProjectRepository                = pgrepo.ProjectRepository
	ProjectTelemetrySourceRepository = pgrepo.ProjectTelemetrySourceRepository
	ProjectUserRoleRepository        = pgrepo.ProjectUserRoleRepository
	RefreshTokenRepository           = pgrepo.RefreshTokenRepository
	RepositoryRepository             = pgrepo.RepositoryRepository
	SetupPlanRepository              = pgrepo.SetupPlanRepository
	SetupTokenRepository             = pgrepo.SetupTokenRepository
	StatusPageRepository             = pgrepo.StatusPageRepository
	SyntheticCheckRepository         = pgrepo.SyntheticCheckRepository
	SyntheticRunnerRepository        = pgrepo.SyntheticRunnerRepository
	TeamRepository                   = pgrepo.TeamRepository
	UserContactMethodRepository      = pgrepo.UserContactMethodRepository
	UserNotificationRuleRepository   = pgrepo.UserNotificationRuleRepository
	UserRepository                   = pgrepo.UserRepository
)
