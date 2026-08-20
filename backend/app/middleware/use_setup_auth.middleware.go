package middleware

import (
	"net/http"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"

	"github.com/gin-gonic/gin"
)

const SetupOrganizationIdContextKey = "setupOrganizationId"
const SetupScopeContextKey = "setupScope"

func UseSetupAuth(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if !strings.HasPrefix(token, "tws_") {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	st, err := transactional.SetupTokenRepository.FindActiveByToken(db.DB, token)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("setup token auth failed: %w", err))
		return
	}
	if st == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.Set(UserIdContextKey, st.UserId)
	c.Set(UserEmailContextKey, st.Email)
	c.Set(SetupOrganizationIdContextKey, st.OrganizationId)
	c.Set(SetupScopeContextKey, true)

	c.Next()
}

func GetSetupOrganizationId(c *gin.Context) int {
	if id, exists := c.Get(SetupOrganizationIdContextKey); exists {
		return id.(int)
	}
	return 0
}

func IsSetupScope(c *gin.Context) bool {
	if v, exists := c.Get(SetupScopeContextKey); exists {
		return v.(bool)
	}
	return false
}
