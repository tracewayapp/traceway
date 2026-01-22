package middleware

import (
	"backend/app/pgdb"
	"backend/app/repositories"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequireProjectAccess middleware validates that the authenticated user has access to the requested project.
// A user can access a project if they are a member of the project's organization.
// This middleware should be applied AFTER UseAppAuth.
var RequireProjectAccess gin.HandlerFunc

func InitRequireProjectAccess() {
	RequireProjectAccess = func(c *gin.Context) {
		userId := GetUserId(c)
		if userId == 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		projectId := extractProjectId(c)
		if projectId == uuid.Nil {
			// you are not authorized to access a no project id, this is just a bad request
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		hasAccess, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
			return repositories.ProjectRepository.UserHasAccess(tx, projectId, userId)
		})

		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if !hasAccess {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		c.Next()
	}
}

func extractProjectId(c *gin.Context) uuid.UUID {
	projectIdStr := c.Query("projectId")
	if projectIdStr != "" {
		if id, err := uuid.Parse(projectIdStr); err == nil {
			return id
		}
	}

	return uuid.Nil
}
