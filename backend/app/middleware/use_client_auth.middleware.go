package middleware

import (
	"backend/app/cache"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const ProjectContextKey = "project"
const ProjectIdContextKey = "project_id"

var ErrProjectIdNotInContext = errors.New("projectId not found in context - ensure RequireProjectAccess middleware is applied")

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
func GetProjectId(c *gin.Context) (uuid.UUID, error) {
	if id, exists := c.Get(ProjectIdContextKey); exists {
		return id.(uuid.UUID), nil
	}
	return uuid.Nil, ErrProjectIdNotInContext
}
