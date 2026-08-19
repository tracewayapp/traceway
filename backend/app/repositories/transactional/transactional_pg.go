//go:build transactional_pg

package transactional

import pgrepo "github.com/tracewayapp/traceway/backend/app/repositories/transactional/pg"

var (
	AuthorizationCodeRepository    = pgrepo.AuthorizationCodeRepository
	CheckIncidentRepository        = pgrepo.CheckIncidentRepository
	CheckRunRepository             = pgrepo.CheckRunRepository
	DashboardRepository            = pgrepo.DashboardRepository
	DashboardTemplateRepository    = pgrepo.DashboardTemplateRepository
	DeviceAuthorizationRepository  = pgrepo.DeviceAuthorizationRepository
	EscalationPolicyRepository     = pgrepo.EscalationPolicyRepository
	IncidentUpdateRepository       = pgrepo.IncidentUpdateRepository
	InvitationRepository           = pgrepo.InvitationRepository
	MetricRegistryRepository       = pgrepo.MetricRegistryRepository
	NotificationChannelRepository  = pgrepo.NotificationChannelRepository
	NotificationRuleRepository     = pgrepo.NotificationRuleRepository
	OncallOverrideRepository       = pgrepo.OncallOverrideRepository
	OncallScheduleRepository       = pgrepo.OncallScheduleRepository
	OAuthSessionRepository         = pgrepo.OAuthSessionRepository
	OauthClientRepository          = pgrepo.OauthClientRepository
	OrganizationRepository         = pgrepo.OrganizationRepository
	OutboxRepository               = pgrepo.OutboxRepository
	PageNotificationRepository     = pgrepo.PageNotificationRepository
	PageRepository                 = pgrepo.PageRepository
	PersonalAccessTokenRepository  = pgrepo.PersonalAccessTokenRepository
	PostMortemRepository           = pgrepo.PostMortemRepository
	ProjectRepository              = pgrepo.ProjectRepository
	ProjectUserRoleRepository      = pgrepo.ProjectUserRoleRepository
	RefreshTokenRepository         = pgrepo.RefreshTokenRepository
	SetupPlanRepository            = pgrepo.SetupPlanRepository
	SetupTokenRepository           = pgrepo.SetupTokenRepository
	StatusPageRepository           = pgrepo.StatusPageRepository
	SyntheticCheckRepository       = pgrepo.SyntheticCheckRepository
	SyntheticRunnerRepository      = pgrepo.SyntheticRunnerRepository
	TeamRepository                 = pgrepo.TeamRepository
	UserContactMethodRepository    = pgrepo.UserContactMethodRepository
	UserNotificationRuleRepository = pgrepo.UserNotificationRuleRepository
	UserRepository                 = pgrepo.UserRepository
)
