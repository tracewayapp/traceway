//go:build transactional_pg

package transactional

import pgrepo "github.com/tracewayapp/traceway/backend/app/repositories/transactional/pg"

var (
	AuthorizationCodeRepository   = pgrepo.AuthorizationCodeRepository
	DeviceAuthorizationRepository = pgrepo.DeviceAuthorizationRepository
	InvitationRepository          = pgrepo.InvitationRepository
	MetricRegistryRepository      = pgrepo.MetricRegistryRepository
	NotificationChannelRepository = pgrepo.NotificationChannelRepository
	NotificationRuleRepository    = pgrepo.NotificationRuleRepository
	OAuthSessionRepository        = pgrepo.OAuthSessionRepository
	OauthClientRepository         = pgrepo.OauthClientRepository
	OrganizationRepository        = pgrepo.OrganizationRepository
	PersonalAccessTokenRepository = pgrepo.PersonalAccessTokenRepository
	ProjectRepository             = pgrepo.ProjectRepository
	RefreshTokenRepository        = pgrepo.RefreshTokenRepository
	UserRepository                = pgrepo.UserRepository
	WidgetGroupRepository         = pgrepo.WidgetGroupRepository
)
