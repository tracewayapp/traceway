package models

import (
	"github.com/tracewayapp/go-lightning/lit"
)

func Init() {
	lit.RegisterModel[Project](lit.PostgreSQL)
	lit.RegisterModel[User](lit.PostgreSQL)
	lit.RegisterModel[Organization](lit.PostgreSQL)
	lit.RegisterModel[OrganizationUser](lit.PostgreSQL)
}
