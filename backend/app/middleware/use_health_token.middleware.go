package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/config"
)

// UseHealthDeepToken guards the operator health endpoint. Its payload is
// instance-wide (cross-tenant queue depth, runner fleet size, storage
// engine stats), so it is gated on the HEALTH_DEEP_TOKEN deploy secret
// rather than any dashboard role; unset disables the endpoint entirely.
func UseHealthDeepToken(c *gin.Context) {
	token := config.Config.HealthDeepToken
	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "The deep health endpoint is disabled: set HEALTH_DEEP_TOKEN"})
		return
	}
	presented, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(presented), []byte(token)) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid health token"})
		return
	}
	c.Next()
}
