package models

import (
	"github.com/tracewayapp/go-lightning/lit"
)

var ExtensionModelRegistrations []func()

func Init() {
	lit.RegisterModel[Project](lit.PostgreSQL)
	lit.RegisterModel[User](lit.PostgreSQL)
	lit.RegisterModel[Organization](lit.PostgreSQL)
	lit.RegisterModel[OrganizationUser](lit.PostgreSQL)
	lit.RegisterModel[OrganizationMember](lit.PostgreSQL)
	lit.RegisterModel[Invitation](lit.PostgreSQL)
	lit.RegisterModel[InvitationWithInviter](lit.PostgreSQL)
	lit.RegisterModel[UserOrganizationResponse](lit.PostgreSQL)
	lit.RegisterModel[CountResult](lit.PostgreSQL)
	lit.RegisterModel[SourceMap](lit.PostgreSQL)

	for _, register := range ExtensionModelRegistrations {
		register()
	}
}
