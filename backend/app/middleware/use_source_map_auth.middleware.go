package middleware

import (
	"backend/app/cache"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var UseSourceMapAuth func(c *gin.Context)

func InitUseSourceMapAuth() {
	UseSourceMapAuth = func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		project := cache.ProjectCache.GetBySourceMapToken(token)
		if project == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Set(ProjectContextKey, project)
		c.Set(ProjectIdContextKey, project.Id)

		c.Next()
	}
}
