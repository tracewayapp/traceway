package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersExemptPublicStatusPages(t *testing.T) {
	prev := appBaseHost
	t.Cleanup(func() { appBaseHost = prev })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders)
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/api/projects", ok)
	r.GET("/api/status/:slug", ok)
	r.NoRoute(ok)

	headers := func(path, host string) http.Header {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = host
		r.ServeHTTP(rec, req)
		return rec.Header()
	}
	framingBlocked := func(h http.Header) bool {
		return h.Get("X-Frame-Options") == "DENY" && strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'")
	}
	framingAllowed := func(h http.Header) bool {
		return h.Get("X-Frame-Options") == "" && !strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors")
	}

	appBaseHost = func() string { return "traceway.example.com" }
	cases := []struct {
		path, host string
		embeddable bool
	}{
		{"/api/projects", "traceway.example.com", false},
		{"/", "traceway.example.com", false},
		{"/issues", "traceway.example.com", false},
		{"/status/api", "traceway.example.com", true},
		{"/api/status/api", "traceway.example.com", true},
		{"/", "status.customer.com", true},
		{"/", "status.customer.com:8443", true},
		{"/issues", "status.customer.com", false},
	}
	for _, tc := range cases {
		h := headers(tc.path, tc.host)
		if tc.embeddable && !framingAllowed(h) {
			t.Errorf("%s on %s: must be embeddable, got X-Frame-Options=%q CSP=%q", tc.path, tc.host, h.Get("X-Frame-Options"), h.Get("Content-Security-Policy"))
		}
		if !tc.embeddable && !framingBlocked(h) {
			t.Errorf("%s on %s: framing must be blocked, got X-Frame-Options=%q CSP=%q", tc.path, tc.host, h.Get("X-Frame-Options"), h.Get("Content-Security-Policy"))
		}
		if h.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(h.Get("Content-Security-Policy"), "base-uri 'self'") {
			t.Errorf("%s on %s: the rest of the policy must survive", tc.path, tc.host)
		}
	}

	appBaseHost = func() string { return "" }
	if !framingBlocked(headers("/", "status.customer.com")) {
		t.Error("without APP_BASE_URL the root must stay unframeable")
	}
	if !framingAllowed(headers("/status/api", "status.customer.com")) {
		t.Error("/status/* must stay embeddable without APP_BASE_URL")
	}
}
