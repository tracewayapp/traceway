package controllers

import (
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/config"
)

// Gin panics at registration when a new path conflicts with an existing
// wildcard, which would take the process down at boot rather than failing any
// handler test. Registering the real tree is the only way to catch it, and
// nothing else in the suite does. The panic is deliberately left uncaught: its
// stack names the conflicting path, which a t.Fatalf would discard.
func TestRouteTreeRegistersWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	if config.Config == nil {
		config.Init(&config.Cfg{})
		t.Cleanup(func() { config.Init(nil) })
	}

	engine := gin.New()
	RegisterControllers(engine.Group("/api"))

	if !slices.ContainsFunc(engine.Routes(), func(r gin.RouteInfo) bool {
		return r.Method == "POST" && r.Path == "/api/organizations"
	}) {
		t.Error("POST /api/organizations was not registered")
	}
}
