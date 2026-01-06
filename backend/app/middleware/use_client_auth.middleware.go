package middleware

import (
	"backend/app/cache"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const ProjectContextKey = "project"
const ProjectIdContextKey = "project_id"

var UseClientAuth func(c *gin.Context)

func InitUseClientAuth() {
	UseClientAuth = func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Extract token from "Bearer <token>"
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Look up project by token in cache
		project := cache.ProjectCache.GetByToken(token)
		if project == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// Set project in context for downstream handlers
		c.Set(ProjectContextKey, project)
		c.Set(ProjectIdContextKey, project.Id)

		c.Next()
	}
}

// GetProjectId retrieves the project ID from the Gin context
func GetProjectId(c *gin.Context) string {
	if id, exists := c.Get(ProjectIdContextKey); exists {
		return id.(string)
	}
	return ""
}
