package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/config"
)

func TestConfigureClientIPRejectsForwardingHeadersFromUntrustedPeers(t *testing.T) {
	router := clientIPRouter(t, &config.Cfg{})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.20")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if got := response.Body.String(); got != "203.0.113.10" {
		t.Fatalf("ClientIP() = %q, want direct peer address", got)
	}
}

func TestConfigureClientIPAcceptsForwardingHeadersFromTrustedPeers(t *testing.T) {
	router := clientIPRouter(t, &config.Cfg{TrustedProxies: "10.0.0.0/8"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.1.2.3:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.20")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if got := response.Body.String(); got != "198.51.100.20" {
		t.Fatalf("ClientIP() = %q, want forwarded client address", got)
	}
}

func TestConfigureClientIPUsesExplicitTrustedPlatformHeader(t *testing.T) {
	router := clientIPRouter(t, &config.Cfg{TrustedProxyHeader: "CF-Connecting-IP"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("CF-Connecting-IP", "198.51.100.20")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if got := response.Body.String(); got != "198.51.100.20" {
		t.Fatalf("ClientIP() = %q, want trusted platform address", got)
	}
}

func TestConfigureClientIPRejectsInvalidTrustedProxy(t *testing.T) {
	router := gin.New()
	if err := configureClientIP(router, &config.Cfg{TrustedProxies: "not-a-cidr"}); err == nil {
		t.Fatal("configureClientIP accepted an invalid proxy")
	}
}

func clientIPRouter(t *testing.T, cfg *config.Cfg) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if err := configureClientIP(router, cfg); err != nil {
		t.Fatalf("configure client IP: %v", err)
	}
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})
	return router
}
