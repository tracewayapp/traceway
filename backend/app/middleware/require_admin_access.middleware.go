package middleware

import (
	"backend/app/pgdb"
	"backend/app/repositories"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

const OrganizationIdContextKey = "organizationId"
const UserOrgRoleContextKey = "userOrgRole"

var RequireAdminAccess gin.HandlerFunc

func InitRequireAdminAccess() {
	RequireAdminAccess = func(c *gin.Context) {
		userId := GetUserId(c)
		if userId == 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		organizationIdStr := c.Param("organizationId")
		organizationId, err := strconv.Atoi(organizationIdStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
			return
		}

		role, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) (string, error) {
			return repositories.OrganizationRepository.GetUserRole(tx, organizationId, userId)
		})

		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to check permissions: %w", err))
			return
		}

		if role != "owner" && role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin or owner access required"})
			return
		}

		c.Set(OrganizationIdContextKey, organizationId)
		c.Set(UserOrgRoleContextKey, role)
		c.Next()
	}
}

func GetOrganizationId(c *gin.Context) int {
	if id, exists := c.Get(OrganizationIdContextKey); exists {
		return id.(int)
	}
	return 0
}

func GetUserOrgRole(c *gin.Context) string {
	if role, exists := c.Get(UserOrgRoleContextKey); exists {
		return role.(string)
	}
	return ""
}
