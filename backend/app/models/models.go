package models

import (
	"github.com/tracewayapp/lit/v2"
)

var ExtensionModelRegistrations []func(lit.Driver)

type metricRegistryNaming struct{ lit.DefaultDbNamingStrategy }

func (metricRegistryNaming) GetTableNameFromStructName(string) string {
	return "metric_registry"
}

// The default pluralizer would produce "escalation_policys".
type escalationPolicyNaming struct{ lit.DefaultDbNamingStrategy }

func (escalationPolicyNaming) GetTableNameFromStructName(string) string {
	return "escalation_policies"
}

// The default pluralizer would produce "outbox_deliveries".
type notificationOutboxNaming struct{ lit.DefaultDbNamingStrategy }

func (notificationOutboxNaming) GetTableNameFromStructName(string) string {
	return "notification_outbox"
}

func Init(driver lit.Driver) {
	lit.RegisterModel[Project](driver)
	lit.RegisterModel[User](driver)
	lit.RegisterModel[Organization](driver)
	lit.RegisterModel[OrganizationUser](driver)
	lit.RegisterModel[OrganizationMember](driver)
	lit.RegisterModel[MemberProjectRole](driver)
	lit.RegisterModel[Invitation](driver)
	lit.RegisterModel[InvitationWithInviter](driver)
	lit.RegisterModel[UserOrganizationResponse](driver)
	lit.RegisterModel[CountResult](driver)
	lit.RegisterModel[SourceMap](driver)
	lit.RegisterModel[SourceMapFlattenMigration](driver)
	lit.RegisterModel[SourceMapProjectId](driver)
	lit.RegisterModelWithNaming[MetricRegistry](driver, metricRegistryNaming{})
	lit.RegisterModel[WidgetGroup](driver)
	lit.RegisterModel[WidgetGroupWidget](driver)
	lit.RegisterModel[WidgetGroupWidgetWithStar](driver)
	lit.RegisterModel[StarredWidget](driver)
	lit.RegisterModel[StarredWidgetWithHome](driver)
	lit.RegisterModel[Dashboard](driver)
	lit.RegisterModel[ProjectDashboard](driver)
	lit.RegisterModel[DashboardAssignment](driver)
	lit.RegisterModel[DashboardTemplate](driver)
	lit.RegisterModel[StarredDashboardWidget](driver)
	lit.RegisterModel[NotificationChannel](driver)
	lit.RegisterModel[NotificationRule](driver)
	lit.RegisterModel[NotificationRuleWithChannel](driver)
	lit.RegisterModel[Team](driver)
	lit.RegisterModel[TeamWithCounts](driver)
	lit.RegisterModel[TeamProjectRow](driver)
	lit.RegisterModel[TeamMember](driver)
	lit.RegisterModel[TeamMemberWithUser](driver)
	lit.RegisterModel[ProjectTeam](driver)
	lit.RegisterModel[OncallSchedule](driver)
	lit.RegisterModel[OncallOverride](driver)
	lit.RegisterModelWithNaming[EscalationPolicy](driver, escalationPolicyNaming{})
	lit.RegisterModel[Page](driver)
	lit.RegisterModel[UserContactMethod](driver)
	lit.RegisterModel[UserNotificationRule](driver)
	lit.RegisterModel[PageNotification](driver)
	lit.RegisterModelWithNaming[OutboxDelivery](driver, notificationOutboxNaming{})
	lit.RegisterModel[OutboxRuleEnqueue](driver)
	lit.RegisterModel[OutboxHealthCounts](driver)
	lit.RegisterModel[SyntheticCheck](driver)
	lit.RegisterModel[CheckRun](driver)
	lit.RegisterModel[CheckRunHealthCounts](driver)
	lit.RegisterModel[CheckIncident](driver)
	lit.RegisterModel[SyntheticRunner](driver)
	lit.RegisterModel[StatusPage](driver)

	for _, register := range ExtensionModelRegistrations {
		register(driver)
	}
}
