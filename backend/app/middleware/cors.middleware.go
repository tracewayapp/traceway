package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const mcpAllowHeaders = "Authorization, Content-Type, Accept, Mcp-Session-Id, Mcp-Protocol-Version, Last-Event-ID"

const reportAllowHeaders = "Content-Type, Content-Encoding, Authorization"

var (
	// Browser SDKs read Retry-After to back off on 503, and the max age keeps a preflight from preceding every export.
	CORSReport    = cors("POST, OPTIONS", reportAllowHeaders, "Retry-After", "7200")
	OpampCors     = cors("POST, OPTIONS", reportAllowHeaders+", OpAMP-Instance-UID", "Retry-After", "7200")
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
