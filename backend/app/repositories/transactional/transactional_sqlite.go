//go:build !transactional_pg

package transactional

import sqliterepo "github.com/tracewayapp/traceway/backend/app/repositories/transactional/sqlite"

var (
	AuthorizationCodeRepository    = sqliterepo.AuthorizationCodeRepository
	CheckIncidentRepository        = sqliterepo.CheckIncidentRepository
	CheckRunRepository             = sqliterepo.CheckRunRepository
	DashboardRepository            = sqliterepo.DashboardRepository
	DashboardTemplateRepository    = sqliterepo.DashboardTemplateRepository
	DeviceAuthorizationRepository  = sqliterepo.DeviceAuthorizationRepository
	EscalationPolicyRepository     = sqliterepo.EscalationPolicyRepository
	InvitationRepository           = sqliterepo.InvitationRepository
	MetricRegistryRepository       = sqliterepo.MetricRegistryRepository
	NotificationChannelRepository  = sqliterepo.NotificationChannelRepository
	NotificationRuleRepository     = sqliterepo.NotificationRuleRepository
	OncallOverrideRepository       = sqliterepo.OncallOverrideRepository
	OncallScheduleRepository       = sqliterepo.OncallScheduleRepository
	OAuthSessionRepository         = sqliterepo.OAuthSessionRepository
	OauthClientRepository          = sqliterepo.OauthClientRepository
	OrganizationRepository         = sqliterepo.OrganizationRepository
	OutboxRepository               = sqliterepo.OutboxRepository
	PageNotificationRepository     = sqliterepo.PageNotificationRepository
	PageRepository                 = sqliterepo.PageRepository
	PersonalAccessTokenRepository  = sqliterepo.PersonalAccessTokenRepository
	ProjectRepository              = sqliterepo.ProjectRepository
	ProjectUserRoleRepository      = sqliterepo.ProjectUserRoleRepository
	RefreshTokenRepository         = sqliterepo.RefreshTokenRepository
	StatusPageRepository           = sqliterepo.StatusPageRepository
	SyntheticCheckRepository       = sqliterepo.SyntheticCheckRepository
	SyntheticRunnerRepository      = sqliterepo.SyntheticRunnerRepository
	TeamRepository                 = sqliterepo.TeamRepository
	UserContactMethodRepository    = sqliterepo.UserContactMethodRepository
	UserNotificationRuleRepository = sqliterepo.UserNotificationRuleRepository
	UserRepository                 = sqliterepo.UserRepository
)
