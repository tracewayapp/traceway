//go:build !transactional_pg

package transactional

import sqliterepo "github.com/tracewayapp/traceway/backend/app/repositories/transactional/sqlite"

var (
	AuthorizationCodeRepository   = sqliterepo.AuthorizationCodeRepository
	DashboardRepository           = sqliterepo.DashboardRepository
	DashboardTemplateRepository   = sqliterepo.DashboardTemplateRepository
	DeviceAuthorizationRepository = sqliterepo.DeviceAuthorizationRepository
	InvitationRepository          = sqliterepo.InvitationRepository
	MetricRegistryRepository      = sqliterepo.MetricRegistryRepository
	NotificationChannelRepository = sqliterepo.NotificationChannelRepository
	NotificationRuleRepository    = sqliterepo.NotificationRuleRepository
	OAuthSessionRepository        = sqliterepo.OAuthSessionRepository
	OauthClientRepository         = sqliterepo.OauthClientRepository
	OrganizationRepository        = sqliterepo.OrganizationRepository
	PersonalAccessTokenRepository = sqliterepo.PersonalAccessTokenRepository
	ProjectRepository             = sqliterepo.ProjectRepository
	ProjectUserRoleRepository     = sqliterepo.ProjectUserRoleRepository
	RefreshTokenRepository        = sqliterepo.RefreshTokenRepository
	UserRepository                = sqliterepo.UserRepository
)
