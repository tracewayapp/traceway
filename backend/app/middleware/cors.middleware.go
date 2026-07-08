package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const mcpAllowHeaders = "Authorization, Content-Type, Accept, Mcp-Session-Id, Mcp-Protocol-Version, Last-Event-ID"

var (
	CORSReport    = cors("POST, OPTIONS", "Content-Type, Content-Encoding, Authorization", "", "")
	MCPCors       = cors("GET, POST, DELETE, OPTIONS", mcpAllowHeaders, "Mcp-Session-Id, WWW-Authenticate", "600")
	WellKnownCors = cors("GET, OPTIONS", mcpAllowHeaders, "", "600")
	OAuthCors     = cors("POST, OPTIONS", mcpAllowHeaders, "", "600")
)

func cors(allowMethods, allowHeaders, exposeHeaders, maxAge string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", allowMethods)
		c.Header("Access-Control-Allow-Headers", allowHeaders)
		if exposeHeaders != "" {
			c.Header("Access-Control-Expose-Headers", exposeHeaders)
		}
		if maxAge != "" {
			c.Header("Access-Control-Max-Age", maxAge)
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
