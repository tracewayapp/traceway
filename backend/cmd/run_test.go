package cmd

import (
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
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

func TestUntrustedForwarderDetectorFiresOnceForPeersOutsideTheList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nets, err := trustedProxyNets([]string{"10.0.0.0/8", "::1"})
	if err != nil {
		t.Fatal(err)
	}
	detect := newUntrustedForwarderDetector(nets)
	check := func(peer, xff string) (string, bool) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Request.RemoteAddr = net.JoinHostPort(peer, "1234")
		if xff != "" {
			c.Request.Header.Set("X-Forwarded-For", xff)
		}
		return detect(c)
	}
	if _, first := check("203.0.113.9", ""); first {
		t.Fatal("fired without a forwarding header")
	}
	if _, first := check("10.1.2.3", "198.51.100.20"); first {
		t.Fatal("fired for a trusted peer")
	}
	if peer, first := check("172.17.0.1", "198.51.100.20"); !first || peer != "172.17.0.1" {
		t.Fatalf("got (%q, %v), want the untrusted peer reported", peer, first)
	}
	if _, first := check("172.17.0.1", "198.51.100.20"); first {
		t.Fatal("fired a second time")
	}
}

func TestTrustedProxyNetsRejectsMalformedEntries(t *testing.T) {
	nets, err := trustedProxyNets([]string{"10.0.0.0/8", "203.0.113.7", "2001:db8::1", "2001:db8::/32"})
	if err != nil {
		t.Fatalf("valid list rejected: %v", err)
	}
	if len(nets) != 4 {
		t.Fatalf("got %d networks, want 4", len(nets))
	}
	if ones, bits := nets[1].Mask.Size(); ones != 32 || bits != 32 {
		t.Fatalf("bare IPv4 mask = /%d of %d, want /32", ones, bits)
	}
	if ones, bits := nets[2].Mask.Size(); ones != 128 || bits != 128 {
		t.Fatalf("bare IPv6 mask = /%d of %d, want /128", ones, bits)
	}
	if _, err := trustedProxyNets([]string{"10.0.0.0/8", "not-a-cidr"}); err == nil {
		t.Fatal("malformed entry accepted")
	}
}

func TestParsePorts(t *testing.T) {
	cases := map[string][]string{
		"80,8082":  {"80", "8082"},
		" 8082 , ": {"8082"},
		",8082":    {"8082"},
		"8082":     {"8082"},
		",":        nil,
		"":         nil,
	}
	for in, want := range cases {
		if got := parsePorts(in); !slices.Equal(got, want) {
			t.Errorf("parsePorts(%q) = %v, want %v", in, got, want)
		}
	}
}
