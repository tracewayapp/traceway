package controllers

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/config"
)

// Gin panics at registration time when a new path conflicts with an existing
// wildcard, which would take the process down at boot rather than failing any
// handler test. Registering the real tree is the only way to catch it, and
// nothing else in the suite does.
func TestRouteTreeRegistersWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	prevConfig := config.Config
	if config.Config == nil {
		config.Init(&config.Cfg{})
	}
	t.Cleanup(func() { config.Init(prevConfig) })

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("route registration panicked: %v", r)
		}
	}()

	engine := gin.New()
	RegisterControllers(engine.Group("/api"))

	// POST /api/organizations is a static sibling of the
	// /api/organizations/:organizationId/... subtree, the shape most likely to
	// trip the wildcard conflict above.
	found := false
	for _, route := range engine.Routes() {
		if route.Method == "POST" && route.Path == "/api/organizations" {
			found = true
			break
		}
	}
	if !found {
		t.Error("POST /api/organizations was not registered")
	}
}
