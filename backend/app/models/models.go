package models

import (
	"github.com/tracewayapp/lit/v2"
)

var ExtensionModelRegistrations []func(lit.Driver)

type metricRegistryNaming struct{ lit.DefaultDbNamingStrategy }

func (metricRegistryNaming) GetTableNameFromStructName(string) string {
	return "metric_registry"
}

func Init(driver lit.Driver) {
	lit.RegisterModel[Project](driver)
	lit.RegisterModel[User](driver)
	lit.RegisterModel[Organization](driver)
	lit.RegisterModel[OrganizationUser](driver)
	lit.RegisterModel[OrganizationMember](driver)
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
	lit.RegisterModel[NotificationChannel](driver)
	lit.RegisterModel[NotificationRule](driver)
	lit.RegisterModel[NotificationRuleWithChannel](driver)

	for _, register := range ExtensionModelRegistrations {
		register(driver)
	}
}
