package models

import (
	"github.com/tracewayapp/go-lightning/lit"
)

func Init() {
	lit.RegisterModel[Project](lit.PostgreSQL)
}
