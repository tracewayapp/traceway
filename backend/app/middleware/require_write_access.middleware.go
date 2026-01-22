package middleware

import (
	"backend/app/cache"
	"backend/app/pgdb"
	"backend/app/repositories"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequireWriteAccess middleware checks if the user has write access to the project's organization.
// It blocks access for users with 'readonly' role.
// This middleware should be applied AFTER UseAppAuth.
var RequireWriteAccess gin.HandlerFunc

func InitRequireWriteAccess() {
	RequireWriteAccess = func(c *gin.Context) {
		userId := GetUserId(c)
		if userId == 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		projectId := extractProjectId(c)
		if projectId == uuid.Nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		project := cache.ProjectCache.GetById(projectId)
		if project == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		if project.OrganizationId == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Project has no organization"})
			return
		}

		role, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) (string, error) {
			return repositories.OrganizationRepository.GetUserRole(tx, *project.OrganizationId, userId)
		})

		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		if role == "readonly" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Read-only access. Write operations are not permitted."})
			return
		}

		c.Next()
	}
}
