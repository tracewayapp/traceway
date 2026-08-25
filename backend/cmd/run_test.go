package cmd

import (
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseTrustedProxies(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want []string
	}{
		{"unset uses the private-range defaults", "", defaultTrustedProxies},
		{"blank uses the defaults", "   ", defaultTrustedProxies},
		{"single entry", "10.0.0.0/8", []string{"10.0.0.0/8"}},
		{"no spaces", "10.0.0.0/8,192.168.0.0/16", []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"space after comma", "10.0.0.0/8, 192.168.0.0/16", []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"trailing comma", "10.0.0.0/8,", []string{"10.0.0.0/8"}},
		{"trailing newline", "10.0.0.0/8\n", []string{"10.0.0.0/8"}},
		{"padded everywhere", "  10.0.0.0/8 , 192.168.0.0/16  ", []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"only separators falls back", " , , ", defaultTrustedProxies},
		{"none trusts no proxy", "none", nil},
		{"none is case-insensitive and padded", "  NONE  ", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTrustedProxies(tc.env)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseTrustedProxiesAlwaysParsesInGin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, env := range []string{"", "10.0.0.0/8, 192.168.0.0/16", "10.0.0.0/8,", "  172.16.0.0/12  ", "fc00::/7,::1"} {
		if err := gin.New().SetTrustedProxies(parseTrustedProxies(env)); err != nil {
			t.Fatalf("TRUSTED_PROXIES=%q produced a list gin rejects: %v", env, err)
		}
	}
}

func clientIPFor(t *testing.T, proxies []string, peer, xff string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, r := gin.CreateTestContext(httptest.NewRecorder())
	if err := r.SetTrustedProxies(proxies); err != nil {
		t.Fatalf("SetTrustedProxies(%v): %v", proxies, err)
	}
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = net.JoinHostPort(peer, "5555")
	c.Request.Header.Set("X-Forwarded-For", xff)
	return c.ClientIP()
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
			if got := clientIPFor(t, defaultTrustedProxies, tc.peer, tc.xff); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
