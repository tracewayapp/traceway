package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func SecurityHeaders(c *gin.Context) {
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/status/") || strings.HasPrefix(path, "/api/status/") {
		c.Header("Content-Security-Policy", "base-uri 'self'; form-action 'self'")
	} else {
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	}
	c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Next()
}
