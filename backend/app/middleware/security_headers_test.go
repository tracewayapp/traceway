package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersExemptPublicStatusPagesFromFraming(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders)
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/api/projects", ok)
	r.GET("/api/status/:slug", ok)
	r.GET("/api/status/:slug/logo", ok)
	r.NoRoute(ok)

	cases := []struct {
		path       string
		embeddable bool
	}{
		{"/api/projects", false},
		{"/", false},
		{"/issues", false},
		{"/status/api", true},
		{"/api/status/api", true},
		{"/api/status/api/logo", true},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		h := rec.Header()
		csp := h.Get("Content-Security-Policy")
		blocked := h.Get("X-Frame-Options") == "DENY" && strings.Contains(csp, "frame-ancestors 'none'")
		allowed := h.Get("X-Frame-Options") == "" && !strings.Contains(csp, "frame-ancestors")
		if tc.embeddable && !allowed {
			t.Errorf("%s: must be embeddable, got X-Frame-Options=%q CSP=%q", tc.path, h.Get("X-Frame-Options"), csp)
		}
		if !tc.embeddable && !blocked {
			t.Errorf("%s: framing must be blocked, got X-Frame-Options=%q CSP=%q", tc.path, h.Get("X-Frame-Options"), csp)
		}
		if h.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(csp, "base-uri 'self'") {
			t.Errorf("%s: the rest of the policy must survive", tc.path)
		}
	}
}
