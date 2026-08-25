package middleware

import (
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/config"
)

func SecurityHeaders(c *gin.Context) {
	if isPublicStatusPage(c) {
		c.Header("Content-Security-Policy", "base-uri 'self'; form-action 'self'")
	} else {
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	}
	c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Next()
}

func isPublicStatusPage(c *gin.Context) bool {
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/status/") || strings.HasPrefix(path, "/api/status/") {
		return true
	}
	if path != "/" {
		return false
	}
	base := appBaseHost()
	return base != "" && requestHost(c.Request.Host) != base
}

var appBaseHost = sync.OnceValue(func() string {
	if config.Config == nil || config.Config.AppBaseURL == "" {
		return ""
	}
	parsed, err := url.Parse(config.Config.AppBaseURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
})

func requestHost(host string) string {
	host = strings.ToLower(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
