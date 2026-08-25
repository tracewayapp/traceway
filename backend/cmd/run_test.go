package cmd

import (
	"net"
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
	if cfg.TrustedProxyHeader != "" {
		router.Use(dropInvalidTrustedProxyHeader(cfg.TrustedProxyHeader))
	}
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})
	return router
}

func TestTrustedProxyListAlwaysParsesInGin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, env := range []string{"", "10.0.0.0/8, 192.168.0.0/16", "10.0.0.0/8,", "  172.16.0.0/12  ", "fc00::/7,::1", "*", "none"} {
		if err := gin.New().SetTrustedProxies((&config.Cfg{TrustedProxies: env}).TrustedProxyList()); err != nil {
			t.Fatalf("TRUSTED_PROXIES=%q produced a list gin rejects: %v", env, err)
		}
	}
}

func clientIPFor(t *testing.T, cfg *config.Cfg, peer, xff string) string {
	t.Helper()
	router := clientIPRouter(t, cfg)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = net.JoinHostPort(peer, "5555")
	req.Header.Set("X-Forwarded-For", xff)
	router.ServeHTTP(rec, req)
	return rec.Body.String()
}

func TestDefaultTrustedProxiesResolveClientIP(t *testing.T) {
	cases := []struct {
		name string
		peer string
		xff  string
		want string
	}{
		{"nginx on a docker bridge forwards the real client", "172.17.0.1", "203.0.113.9", "203.0.113.9"},
		{"proxy on an IPv6 ULA network forwards the real client", "fd00::1", "203.0.113.9", "203.0.113.9"},
		{"loopback proxy forwards the real client", "127.0.0.1", "203.0.113.9", "203.0.113.9"},
		{"two hops, inner private", "172.17.0.1", "203.0.113.9, 10.0.0.7", "203.0.113.9"},
		{"direct internet client cannot spoof", "203.0.113.50", "1.2.3.4", "203.0.113.50"},
		{"CGNAT peer cannot spoof", "100.100.5.5", "1.2.3.4", "100.100.5.5"},
		{"spoof through a trusted proxy stops at the untrusted hop", "172.17.0.1", "9.9.9.9, 203.0.113.9", "203.0.113.9"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clientIPFor(t, &config.Cfg{}, tc.peer, tc.xff); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTrustedProxyHeaderIgnoresValuesThatAreNotAnIP(t *testing.T) {
	cfg := &config.Cfg{TrustedProxyHeader: "CF-Connecting-IP"}
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"valid IPv4", "198.51.100.20", "198.51.100.20"},
		{"valid IPv6", "2001:db8::1", "2001:db8::1"},
		{"garbage falls back to the peer", "not-an-ip", "203.0.113.10"},
		{"a list falls back to the peer", "1.2.3.4, 5.6.7.8", "203.0.113.10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := clientIPRouter(t, cfg)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "203.0.113.10:1234"
			req.Header.Set("CF-Connecting-IP", tc.value)
			router.ServeHTTP(rec, req)
			if got := rec.Body.String(); got != tc.want {
				t.Fatalf("ClientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}
